package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type fakeDynamoScanner struct {
	output *dynamodb.ScanOutput
	err    error
	input  *dynamodb.ScanInput
}

func (f *fakeDynamoScanner) Scan(_ context.Context, input *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	f.input = input
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

func TestLoad_UsesYAMLWhenTableNameEmpty(t *testing.T) {
	cfgPath := writeYAMLConfig(t, `
repos:
  - name: acme/repo
    environments: [dev]
    enabled: true
`)

	repos, err := Load(context.Background(), "", cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "acme/repo" {
		t.Fatalf("unexpected repos: %+v", repos)
	}
}

func TestLoad_WrapsDynamoLoaderCreationError(t *testing.T) {
	orig := newDynamoLoader
	newDynamoLoader = func(context.Context, string) (*DynamoLoader, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { newDynamoLoader = orig })

	_, err := Load(context.Background(), "runner-config", "")
	if err == nil || err.Error() != `creating dynamo loader: boom` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDynamoLoader_WrapsAWSConfigError(t *testing.T) {
	orig := loadDefaultAWSConfig
	loadDefaultAWSConfig = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, errors.New("aws-config")
	}
	t.Cleanup(func() { loadDefaultAWSConfig = orig })

	_, err := NewDynamoLoader(context.Background(), "cfg")
	if err == nil || err.Error() != `loading AWS config: aws-config` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDynamoLoader_Success(t *testing.T) {
	orig := loadDefaultAWSConfig
	loadDefaultAWSConfig = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{Region: "us-east-1"}, nil
	}
	t.Cleanup(func() { loadDefaultAWSConfig = orig })

	loader, err := NewDynamoLoader(context.Background(), "cfg")
	if err != nil {
		t.Fatalf("NewDynamoLoader() unexpected error: %v", err)
	}
	if loader == nil || loader.client == nil || loader.tableName != "cfg" {
		t.Fatalf("unexpected loader: %+v", loader)
	}
}

func TestDynamoLoader_Load_FiltersEnabledRepos(t *testing.T) {
	scanner := &fakeDynamoScanner{
		output: &dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{
					"PK":     &types.AttributeValueMemberS{Value: "RUNNER#repos"},
					"config": &types.AttributeValueMemberS{Value: `{"Name":"acme/repo","Environments":["dev"],"Enabled":true}`},
				},
				{
					"PK":     &types.AttributeValueMemberS{Value: "RUNNER#repos"},
					"config": &types.AttributeValueMemberS{Value: `{"Name":"acme/disabled","Enabled":false}`},
				},
			},
		},
	}

	loader := &DynamoLoader{client: scanner, tableName: "runner-config"}
	repos, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "acme/repo" {
		t.Fatalf("unexpected repos: %+v", repos)
	}
	if scanner.input == nil || aws.ToString(scanner.input.TableName) != "runner-config" {
		t.Fatalf("unexpected scan input: %+v", scanner.input)
	}
}

func TestDynamoLoader_Load_ScanError(t *testing.T) {
	loader := &DynamoLoader{
		client:    &fakeDynamoScanner{err: errors.New("scan failed")},
		tableName: "runner-config",
	}

	_, err := loader.Load(context.Background())
	if err == nil || err.Error() != `scanning dynamodb table "runner-config": scan failed` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDynamoLoader_Load_UnmarshalError(t *testing.T) {
	loader := &DynamoLoader{
		client: &fakeDynamoScanner{
			output: &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{
					{"config": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}}},
				},
			},
		},
		tableName: "runner-config",
	}

	_, err := loader.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unmarshalling dynamodb item:") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDynamoLoader_Load_InvalidJSON(t *testing.T) {
	loader := &DynamoLoader{
		client: &fakeDynamoScanner{
			output: &dynamodb.ScanOutput{
				Items: []map[string]types.AttributeValue{
					{
						"PK":     &types.AttributeValueMemberS{Value: "RUNNER#repos"},
						"config": &types.AttributeValueMemberS{Value: `{`},
					},
				},
			},
		},
		tableName: "runner-config",
	}

	_, err := loader.Load(context.Background())
	if err == nil || err.Error() != `parsing config JSON for item: unexpected end of JSON input` {
		t.Fatalf("unexpected error: %v", err)
	}
}
