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
	log "github.com/sirupsen/logrus"
)

type LLMClient interface {
	GenerateProblems(ctx context.Context, docStr string) ([]view.AIApiDocProblem, error)
	CategorizeProblems(ctx context.Context, problems []view.AIApiDocProblem) ([]view.AIApiDocCatProblem, error)
	FixProblems(ctx context.Context, docStr string, problems []view.AIApiDocCatProblem, lintReport []view.ValidationIssue) (string, error)
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
		client:                 openai.NewClient(opts...),
		model:                  openAIModel,
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
What it measures: The presence, quality, and usefulness of the description fields for paths, operations, parameters, and response schemas.
LLM Analysis: The LLM can check for the existence of descriptions and then evaluate their quality. A good description explains the "why," not just the "what." For example, "userId (string)" is poor, while "userId (string): The unique identifier of the user, retrieved from the authentication apiKey" is excellent.
Scoring: A score based on the percentage of elements with descriptions and the semantic richness of those descriptions (e.g., length, use of key terms, clarity).
2. Usefulness and Accuracy of Examples
What it measures: The presence and realism of example or examples fields in schemas and parameters.
LLM Analysis: An LLM can determine if examples are provided and if they are logically consistent with the schema definition. For instance, an example for a dateOfBirth parameter should be a valid date string, not a random integer. It can also judge if an example is a realistic, edge-case, or just a trivial placeholder.
Scoring: Score based on the coverage of examples across the API and the contextual accuracy and realism of each example.
3. Logical Consistency and Naming Conventions
What it measures: The consistency in naming paths, parameters, and schema properties, and the logical grouping of resources.
LLM Analysis: The LLM can detect inconsistencies like mixed naming schemes (e.g., /getUsers vs. /products/{id}), or incoherent pluralization (e.g., /user and /users in the same API). It can also assess if resource hierarchies make sense (e.g., /orgs/{org_id}/users/{user_id}/posts is logical).
Scoring: A score reflecting the uniformity of naming and the logical flow of resource relationships. Severity should not be higher than warning.
4. Error Handling Comprehensiveness
What it measures: The extent to which the API defines non-success HTTP status codes (4xx, 5xx) and their corresponding error response schemas.
LLM Analysis: The LLM can check if operations define common error responses like 400 Bad Request, 401 Unauthorized, 404 Not Found, and 500 Internal Server Error. A higher score is given if these responses have a structured schema (e.g., using a Error component) with descriptive fields like code, message, and details.
Scoring: A score based on the coverage of expected error codes across operations and the richness of the defined error schemas.
5. Schema Reusability and Structure
What it measures: The effective use of OpenAPI components ($ref) to avoid duplication and promote a consistent data model.
LLM Analysis: The LLM can analyze the components/schemas section to identify duplicated structures that should be refactored into a shared definition. It can assess if the schemas are well-normalized and if common objects (like User, Error, PaginationMetadata) are defined once and reused.
Scoring: A score based on the ratio of reused components ($ref) to inline schemas and the level of duplication detected. Severity should not be higher than warning.
6. Security Schema Clarity
What it measures: The clarity and detail provided in the components/securitySchemes definition.
LLM Analysis: Beyond just having a security scheme defined (e.g., type: http, scheme: bearer), the LLM can evaluate the quality of the description field. A high-quality description explains the apiKey format, how to obtain it (e.g., link to an auth server), and any required scopes or flows.
Scoring: A score based on the presence and comprehensiveness of the security scheme descriptions.

Severity in deprecated operations should not be higher than warning.
When determining the entity name, use TMF SID and TMF Open API notation, selecting names that align with these specifications when applicable.
List identified issues in json format. Avoid any other output.`

func (l OAIClientImpl) GenerateProblems(ctx context.Context, docStr string) ([]view.AIApiDocProblem, error) {
	start := time.Now()
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

	log.Infof("run detect problems with openai client")

	chat, err := l.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
		Model: l.model,
	})
	log.Infof("finished detect problems with openai client, it took %dms", time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}

	var result view.IAProblemsOutput
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &result)
	if err != nil {
		return nil, err
	}

	return result.Problems, nil
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

	log.Infof("run categorize problems with openai client")

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

const defaultFixProblemsPrompt = `You need to enhance the specification and fix the following problems. Consider list of problems and linter report.
Do not rename tags. Do not change paths and parameters. Do not introduce breaking changes.
Use TMF SID and TMF Open API notation, selecting names that align with these specifications when applicable.
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
