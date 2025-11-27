package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/utils"
	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	log "github.com/sirupsen/logrus"
)

type LLMClient interface {
	GenerateProblems(ctx context.Context, docStr string) ([]view.AIApiDocProblem, string, error)
	CategorizeProblems(ctx context.Context, problems []view.AIApiDocProblem) ([]view.AIApiDocCatProblem, error)
	FixProblems(ctx context.Context, docStr string, problems []view.AIApiDocCatProblem, lintReport []view.ValidationIssue) (string, error)
	MergeProblems(ctx context.Context, problems []view.AIApiDocCatProblem) ([]view.AIApiDocCatProblem, error)
	UpdateGenerateProblemsPrompt(prompt string)
	UpdateFixProblemsPrompt(prompt string)
	UpdateModel(model string) error
}

func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, 1800*time.Second)
}

func NewOpenaiClient(apiKey string, model string, proxy string) (LLMClient, error) {

	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	} else {
		return nil, errors.New("openai: api key is required")
	}

	if proxy != "" {
		// TODO: validate URL
		opts = append(opts, option.WithBaseURL(proxy))
	}

	var openAIModel openai.ChatModel
	if model != "" {
		// TODO: validate the model!
		openAIModel = model
	} else {
		openAIModel = openai.ChatModelGPT5
	}

	tr := http.Transport{
		Dial:                  dialTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout:   time.Second * 1800,
		IdleConnTimeout:       time.Second * 1800,
		ResponseHeaderTimeout: time.Second * 1800,
		ExpectContinueTimeout: time.Second * 1800,
	}
	cl := http.Client{Transport: &tr, Timeout: time.Second * 1800}

	opts = append(opts, option.WithHTTPClient(&cl))

	return &OAIClientImpl{
		client: openai.NewClient(opts...),
		model:  openAIModel,

		generateProblemsPrompt: defaultGenerateProblemsPrompt,
		fixProblemsPrompt:      defaultFixProblemsPrompt,
	}, nil
}

type OAIClientImpl struct {
	client openai.Client
	model  openai.ChatModel

	generateProblemsPrompt string
	fixProblemsPrompt      string
}

var IAProblemsOutputResponseSchema = GenerateSchema[view.IAProblemsOutput]()
var IACatProblemsOutputResponseSchema = GenerateSchema[view.AIApiDocCatProblemsOutput]()

const defaultGenerateProblemsPrompt = `You need to analyze the following OpenApi document by the following criteria:
1. Clarity and Completeness of Descriptions
What it measures: The presence, quality, and usefulness of the description fields for paths, operations, parameters, response schemas, etc.
Check for the existence of descriptions and then evaluate their quality.

2. Usefulness and Accuracy of Examples
What it measures: The presence and realism of example or examples fields in schemas, parameters, etc.
Check if examples are provided and if they are logically consistent with the schema definition. 
For instance, an example for a dateOfBirth parameter should be a valid date string(considering format), not a random integer. 


Severity in deprecated operations should not be higher than warning.
List identified issues in json format. Avoid any other output.`

func (l OAIClientImpl) GenerateProblems(ctx context.Context, docStr string) ([]view.AIApiDocProblem, string, error) {
	// TODO: parametrization?
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(defaultGenerateProblemsPrompt),
		openai.UserMessage(docStr),
	}

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:   "problems_result",
		Schema: IAProblemsOutputResponseSchema,
		Strict: openai.Bool(true),
	}

	reasoningEffort := ""
	if l.model == openai.ChatModelGPT5 {
		reasoningEffort = string(openai.ReasoningEffortMinimal)
	}

	chat, err := l.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{

		Messages: messages,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
		ReasoningEffort: openai.ReasoningEffort(reasoningEffort),
		//Verbosity:       openai.ChatCompletionNewParamsVerbosityLow,
		Model: l.model,
	})

	if err != nil {
		return nil, "", err
	}

	var result view.IAProblemsOutput
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &result)
	if err != nil {
		return nil, "", err
	}

	promptHash := utils.CreateSHA256Hash([]byte(defaultGenerateProblemsPrompt))

	return result.Problems, promptHash, nil
}

func (l OAIClientImpl) CategorizeProblems(ctx context.Context, problems []view.AIApiDocProblem) ([]view.AIApiDocCatProblem, error) {
	start := time.Now()
	problemsBytes, err := json.MarshalIndent(problems, "", "    ")
	if err != nil {
		return nil, err
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`You need to categorize the problems. Avoid any other output.`),
		openai.UserMessage("problems: \n" + string(problemsBytes)),
	}

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:   "categorized_problems_result",
		Schema: IACatProblemsOutputResponseSchema,
		Strict: openai.Bool(true),
	}

	chat, err := l.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
		Model: l.model,
	})
	log.Infof("finished categorize problems with openai client, it took %dms", time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}

	var result view.AIApiDocCatProblemsOutput
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &result)
	if err != nil {
		return nil, err
	}

	return result.Problems, nil
}

const defaultFixProblemsPrompt = `You need to enhance the specification and fix the following problems. 
Consider list of problems and linter report. Do not rename tags. 
Do not change paths and parameters. Use TMF Open API notation when applicable. 
Return only updated specification (with changes). Avoid any other output.`

func (l OAIClientImpl) FixProblems(ctx context.Context, docStr string, problems []view.AIApiDocCatProblem, lintReport []view.ValidationIssue) (string, error) {
	problemsBytes, err := json.MarshalIndent(problems, "", "    ")
	if err != nil {
		return "", err
	}

	linterReportBytes, err := json.MarshalIndent(lintReport, "", "    ")
	if err != nil {
		return "", err
	}
	// TODO: parametrization?
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(defaultFixProblemsPrompt),

		openai.UserMessage("problems: \n" + string(problemsBytes)),
		openai.UserMessage("linter report: \n" + string(linterReportBytes)),
		openai.UserMessage("specification: \n" + docStr),
	}

	log.Infof("run fix problems with openai client")

	chat, err := l.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,

		Model: l.model,
	})
	if err != nil {
		return "", err
	}

	return chat.Choices[0].Message.Content, nil
}

const defaultMergeProblemsPrompt = "Merge similar problems"

func (l OAIClientImpl) MergeProblems(ctx context.Context, problems []view.AIApiDocCatProblem) ([]view.AIApiDocCatProblem, error) {
	problemsStr, err := json.Marshal(problems)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal problems, error: %v", err)
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(defaultMergeProblemsPrompt),
		openai.UserMessage(string(problemsStr)),
	}

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:   "problems_result",
		Schema: IACatProblemsOutputResponseSchema,
		Strict: openai.Bool(true),
	}

	reasoningEffort := ""
	if l.model == openai.ChatModelGPT5 {
		reasoningEffort = string(openai.ReasoningEffortMinimal)
	}

	chat, err := l.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
		ReasoningEffort: openai.ReasoningEffort(reasoningEffort),
		//Verbosity:       openai.ChatCompletionNewParamsVerbosityLow,
		Model: l.model,
	})

	if err != nil {
		return nil, err
	}

	var result view.AIApiDocCatProblemsOutput
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &result)
	if err != nil {
		return nil, err
	}

	return result.Problems, nil
}

func GenerateSchema[T any]() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

func (l OAIClientImpl) UpdateGenerateProblemsPrompt(prompt string) {
	l.generateProblemsPrompt = prompt
}

func (l OAIClientImpl) UpdateFixProblemsPrompt(prompt string) {
	l.fixProblemsPrompt = prompt
}

func (l OAIClientImpl) UpdateModel(model string) error {
	l.model = model
	return nil
}
