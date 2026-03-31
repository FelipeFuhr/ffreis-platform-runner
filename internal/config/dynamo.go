package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type dynamoScanner interface {
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

// DynamoLoader loads RepoConfig items from a DynamoDB table.
// Items must have PK = "RUNNER#repos" and a "config" attribute containing JSON.
type DynamoLoader struct {
	client    dynamoScanner
	tableName string
}

var loadDefaultAWSConfig = awsconfig.LoadDefaultConfig

// NewDynamoLoader creates a DynamoLoader using the default AWS SDK config.
func NewDynamoLoader(ctx context.Context, tableName string) (*DynamoLoader, error) {
	cfg, err := loadDefaultAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	client := dynamodb.NewFromConfig(cfg)
	return &DynamoLoader{client: client, tableName: tableName}, nil
}

// dynamoItem represents a raw DynamoDB item with PK and config fields.
type dynamoItem struct {
	PK     string `dynamodbav:"PK"`
	Config string `dynamodbav:"config"`
}

// Load scans the DynamoDB table for items with PK = "RUNNER#repos".
func (d *DynamoLoader) Load(ctx context.Context) ([]RepoConfig, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(d.tableName),
		FilterExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "RUNNER#repos"},
		},
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("scanning dynamodb table %q: %w", d.tableName, err)
	}

	var configs []RepoConfig
	for _, item := range result.Items {
		var di dynamoItem
		if err := attributevalue.UnmarshalMap(item, &di); err != nil {
			return nil, fmt.Errorf("unmarshalling dynamodb item: %w", err)
		}

		var rc RepoConfig
		if err := json.Unmarshal([]byte(di.Config), &rc); err != nil {
			return nil, fmt.Errorf("parsing config JSON for item: %w", err)
		}

		if rc.Enabled {
			configs = append(configs, rc)
		}
	}

	return configs, nil
}
