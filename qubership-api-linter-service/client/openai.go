package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/Netcracker/qubership-api-linter-service/view"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type LLMClient interface {
	LintOasDocument(ctx context.Context, docStr string, prompt string) ([]view.AiValidationIssue, error)
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
	}, nil
}

type OAIClientImpl struct {
	client openai.Client
	model  openai.ChatModel

	generateProblemsPrompt string
	fixProblemsPrompt      string
}

var AiValidationIssuesOutputResponseSchema = generateSchema[view.AiValidationIssuesOutput]()

func (o OAIClientImpl) LintOasDocument(ctx context.Context, docStr string, prompt string) ([]view.AiValidationIssue, error) {

	// TODO: client side rate limiter to control token usage and avoid errors

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(prompt),
		openai.UserMessage(docStr),
	}

	//Invalid schema for response_format 'problems_result': In context=('properties', 'issues', 'items'), 'required' is required to be supplied and to be an array including every key in properties. Missing 'path'.

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:   "oas_doc_lint_response",
		Schema: AiValidationIssuesOutputResponseSchema,
		Strict: openai.Bool(true),
	}

	reasoningEffort := ""
	if o.model == openai.ChatModelGPT5 {
		reasoningEffort = string(openai.ReasoningEffortMinimal)
	}

	chat, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{

		Messages: messages,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
		ReasoningEffort: openai.ReasoningEffort(reasoningEffort),
		//Verbosity:       openai.ChatCompletionNewParamsVerbosityLow,
		Model: o.model,
	})

	if err != nil {
		return nil, err
	}

	var result view.AiValidationIssuesOutput
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &result)
	if err != nil {
		return nil, err
	}

	return result.Issues, nil
}

func generateSchema[T any]() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}
