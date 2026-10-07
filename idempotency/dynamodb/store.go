package dynamodb

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	sdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	"github.com/rambow-cloud/powertools-lambda-go/internal/jsonvalue"
)

type Client interface {
	PutItem(context.Context, *sdk.PutItemInput, ...func(*sdk.Options)) (*sdk.PutItemOutput, error)
	GetItem(context.Context, *sdk.GetItemInput, ...func(*sdk.Options)) (*sdk.GetItemOutput, error)
	UpdateItem(context.Context, *sdk.UpdateItemInput, ...func(*sdk.Options)) (*sdk.UpdateItemOutput, error)
	DeleteItem(context.Context, *sdk.DeleteItemInput, ...func(*sdk.Options)) (*sdk.DeleteItemOutput, error)
}

// Options maps the reference attribute names. A sort key enables a static
// partition key, defaulting to idempotency#<AWS_LAMBDA_FUNCTION_NAME>.
type Options struct {
	TableName                                                   string
	KeyAttribute, SortKeyAttribute, StaticPartitionKey          string
	StatusAttribute, ExpiryAttribute, InProgressExpiryAttribute string
	DataAttribute, ValidationAttribute                          string
}

type Store struct {
	client  Client
	options Options
}

func New(client Client, options Options) (*Store, error) {
	if client == nil || strings.TrimSpace(options.TableName) == "" {
		return nil, &idempotency.ConfigurationError{Message: "DynamoDB client and table name are required"}
	}
	defaults := []struct {
		value    *string
		fallback string
	}{
		{&options.KeyAttribute, "id"}, {&options.StatusAttribute, "status"},
		{&options.ExpiryAttribute, "expiration"}, {&options.InProgressExpiryAttribute, "in_progress_expiration"},
		{&options.DataAttribute, "data"}, {&options.ValidationAttribute, "validation"},
	}
	seen := map[string]bool{}
	for _, field := range defaults {
		if *field.value == "" {
			*field.value = field.fallback
		}
		if seen[*field.value] {
			return nil, &idempotency.ConfigurationError{Message: "DynamoDB attribute names must be distinct"}
		}
		seen[*field.value] = true
	}
	if options.SortKeyAttribute != "" && seen[options.SortKeyAttribute] {
		return nil, &idempotency.ConfigurationError{Message: "DynamoDB sort key must be distinct from other attributes"}
	}
	if options.StaticPartitionKey == "" {
		name, _ := commons.StringEnv("AWS_LAMBDA_FUNCTION_NAME", "")
		options.StaticPartitionKey = "idempotency#" + name
	}
	return &Store{client, options}, nil
}

func identity(options *sdk.Options) {
	options.APIOptions = append(options.APIOptions, awssdk.UserAgent("idempotency"))
}
func text(value string) types.AttributeValue { return &types.AttributeValueMemberS{Value: value} }
func number(value int64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(value, 10)}
}

func (s *Store) key(key string) map[string]types.AttributeValue {
	if s.options.SortKeyAttribute == "" {
		return map[string]types.AttributeValue{s.options.KeyAttribute: text(key)}
	}
	return map[string]types.AttributeValue{s.options.KeyAttribute: text(s.options.StaticPartitionKey), s.options.SortKeyAttribute: text(key)}
}

func (s *Store) Put(ctx context.Context, record idempotency.Record, now time.Time) error {
	o := s.options
	item := s.key(record.Key)
	item[o.StatusAttribute] = text(string(record.Status))
	item[o.ExpiryAttribute] = number(record.Expiration)
	if record.InProgressExpiration != 0 {
		item[o.InProgressExpiryAttribute] = number(record.InProgressExpiration)
	}
	if record.Validation != "" {
		item[o.ValidationAttribute] = text(record.Validation)
	}
	_, err := s.client.PutItem(ctx, &sdk.PutItemInput{
		TableName: aws.String(o.TableName), Item: item,
		ConditionExpression:                 aws.String("attribute_not_exists(#id) OR #expiry < :now OR (#status = :inprogress AND attribute_exists(#in_progress_expiry) AND #in_progress_expiry < :now_in_millis)"),
		ExpressionAttributeNames:            map[string]string{"#id": o.KeyAttribute, "#expiry": o.ExpiryAttribute, "#status": o.StatusAttribute, "#in_progress_expiry": o.InProgressExpiryAttribute},
		ExpressionAttributeValues:           map[string]types.AttributeValue{":now": &types.AttributeValueMemberN{Value: strconv.FormatFloat(float64(now.UnixMilli())/1000, 'f', -1, 64)}, ":now_in_millis": number(now.UnixMilli()), ":inprogress": text(string(idempotency.InProgress))},
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	}, identity)
	var conflict *types.ConditionalCheckFailedException
	if !errors.As(err, &conflict) {
		return err
	}
	if len(conflict.Item) == 0 {
		return &idempotency.AlreadyExistsError{}
	}
	existing, decodeErr := s.decode(record.Key, conflict.Item)
	if decodeErr != nil {
		return decodeErr
	}
	return &idempotency.AlreadyExistsError{Record: &existing}
}

func (s *Store) Get(ctx context.Context, key string) (idempotency.Record, error) {
	output, err := s.client.GetItem(ctx, &sdk.GetItemInput{TableName: aws.String(s.options.TableName), Key: s.key(key), ConsistentRead: aws.Bool(true)}, identity)
	if err != nil {
		return idempotency.Record{}, err
	}
	if output == nil || len(output.Item) == 0 {
		return idempotency.Record{}, idempotency.ErrNotFound
	}
	return s.decode(key, output.Item)
}

func (s *Store) Update(ctx context.Context, record idempotency.Record) error {
	o := s.options
	names := map[string]string{"#expiry": o.ExpiryAttribute, "#status": o.StatusAttribute}
	values := map[string]types.AttributeValue{":expiry": number(record.Expiration), ":status": text(string(record.Status))}
	fields := []string{"#expiry = :expiry", "#status = :status"}
	if len(record.Data) > 0 {
		var value any
		if err := json.Unmarshal(record.Data, &value, jsonvalue.Numbers); err != nil {
			return err
		}
		attribute, err := attributevalue.Marshal(value)
		if err != nil {
			return err
		}
		names["#response_data"] = o.DataAttribute
		values[":response_data"] = attribute
		fields = append(fields, "#response_data = :response_data")
	}
	if record.Validation != "" {
		names["#validation_key"] = o.ValidationAttribute
		values[":validation_key"] = text(record.Validation)
		fields = append(fields, "#validation_key = :validation_key")
	}
	_, err := s.client.UpdateItem(ctx, &sdk.UpdateItemInput{TableName: aws.String(o.TableName), Key: s.key(record.Key), UpdateExpression: aws.String("SET " + strings.Join(fields, ", ")), ExpressionAttributeNames: names, ExpressionAttributeValues: values}, identity)
	return err
}

func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteItem(ctx, &sdk.DeleteItemInput{TableName: aws.String(s.options.TableName), Key: s.key(key)}, identity)
	return err
}

func (s *Store) decode(key string, item map[string]types.AttributeValue) (idempotency.Record, error) {
	o := s.options
	r := idempotency.Record{Key: key}
	status, ok := item[o.StatusAttribute].(*types.AttributeValueMemberS)
	if !ok {
		return r, fmt.Errorf("missing or invalid status attribute")
	}
	r.Status = idempotency.Status(status.Value)
	for _, field := range []struct {
		name   string
		target *int64
	}{{o.ExpiryAttribute, &r.Expiration}, {o.InProgressExpiryAttribute, &r.InProgressExpiration}} {
		if value, exists := item[field.name]; exists {
			n, ok := value.(*types.AttributeValueMemberN)
			if !ok {
				return r, fmt.Errorf("invalid timestamp attribute %s", field.name)
			}
			parsed, err := strconv.ParseInt(n.Value, 10, 64)
			if err != nil {
				return r, err
			}
			*field.target = parsed
		}
	}
	if value, exists := item[o.ValidationAttribute]; exists {
		v, ok := value.(*types.AttributeValueMemberS)
		if !ok {
			return r, fmt.Errorf("invalid validation attribute")
		}
		r.Validation = v.Value
	}
	if value, exists := item[o.DataAttribute]; exists {
		var response any
		decoder := attributevalue.NewDecoder(func(o *attributevalue.DecoderOptions) { o.UseNumber = true })
		if err := decoder.Decode(value, &response); err != nil {
			return r, err
		}
		// SDK Number is a string alias; convert recursively to JSON numbers.
		encoded, err := json.Marshal(jsonNumbers(response))
		if err != nil {
			return r, err
		}
		r.Data = encoded
	}
	return r, nil
}

func jsonNumbers(value any) any {
	switch v := value.(type) {
	case attributevalue.Number:
		return jsonv1.Number(v)
	case map[string]any:
		for key, item := range v {
			v[key] = jsonNumbers(item)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = jsonNumbers(item)
		}
		return v
	default:
		return value
	}
}

var _ idempotency.Store = (*Store)(nil)
