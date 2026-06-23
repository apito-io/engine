package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"golang.org/x/crypto/bcrypt"
)

func (s *GraphQLServer) HandlePayloadFormattingOld(ctx context.Context, param *models.CommonSystemParams, payload map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {

	input := param.ResolveParams.Args
	var inputPayload map[string]interface{}
	if val, ok := input["payload"].(map[string]interface{}); ok {
		inputPayload = val
	}

	// local support
	local := "en"
	if val, ok := input["local"].(string); ok {
		local = val
	}

	oldPayload := make(map[string]interface{})
	modifiedPayloads := make(map[string]interface{})

	for _, f := range param.Model.Fields {
		val := payload[f.Identifier]
		if newVal, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && f.InputType == "geo" {
			lat := newVal["lat"].(float64)
			lon := newVal["lon"].(float64)
			modifiedPayloads[f.Identifier] = map[string]interface{}{
				"lat":         lat,
				"lon":         lon,
				"type":        "Point",
				"coordinates": []float64{lat, lon},
			}
		} else if pass, ok := inputPayload["secret"].(string); ok && param.Model.Name == "user" && f.Identifier == "secret" { // check for password payload.
			hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
			if err != nil {
				return nil, nil, errors.New("Internal Error while saving secret")
			}
			modifiedPayloads[f.Identifier] = string(hash)

		} else if media := inputPayload[f.Identifier]; media != nil && f.FieldType == "media" {
			var err error
			switch media.(type) {
			case []interface{}:
				var result []interface{}
				for _, media := range media.([]interface{}) {
					var m interface{}
					//m, err = s.GraphQLExecutor.HandleMediaURL(ctx, param, media.(map[string]interface{}))
					result = append(result, m)
					fmt.Println(media)
				}
				if len(result) > 0 {
					modifiedPayloads[f.Identifier] = result
				}
			case interface{}:
				if media, ok := media.(map[string]interface{}); ok {
					//modifiedPayloads[f.Identifier], err = s.GraphQLExecutor.HandleMediaURL(ctx, param, media)
					fmt.Println(media)
				}
			}
			if err != nil {
				return nil, nil, err
			}
		} else if local != "en" && f.Validation != nil && utility.ArrayContains(f.Validation.Locals, local) { // handle Local First the local support
			modifiedPayloads[fmt.Sprintf(`%s_%s`, f.Identifier, local)] = payload[f.Identifier]
			// no need for original field to have data , ex : if title_bn is present then no need for title in payload
			oldPayload[f.Identifier] = val
			delete(modifiedPayloads, f.Identifier)
		} else if f.FieldType == "repeated" && len(f.SubFieldInfo) > 0 {
			if data, ok := inputPayload[f.Identifier].([]interface{}); ok {
				var results []map[string]interface{}
				for _, d := range data {
					if len(d.(map[string]interface{})) > 0 {

						//results = append(results, result)
					}
				}
				modifiedPayloads[f.Identifier] = results
			}
		} else if f.FieldType == "multiline" {
			if val, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(val) > 0 {
				// Use the new markdown processor to handle multiline fields
				processed := utility.ProcessMultilineField(val)
				modifiedPayloads[f.Identifier] = processed
			}
		} else if p, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(p) > 0 && f.FieldType == "repeated" {
			if id, ok := p["_id"].(string); ok {
				for i, old := range modifiedPayloads[f.Identifier].([]interface{}) {
					if old.(map[string]interface{})["_id"] == id {
						modifiedPayloads[f.Identifier].([]interface{})[i] = p
					}
				}
			} else {
				id := utility.NewID()
				p["_id"] = id
				modifiedPayloads[f.Identifier] = append(modifiedPayloads[f.Identifier].([]interface{}), p)
			}
		} else if p, ok := inputPayload[f.Identifier].(float64); ok && f.InputType == "int" {
			_int := int(p)
			if _int > 2147483647 {
				return nil, nil, errors.New(fmt.Sprintf("Int Value Overflow for `%s`. Field type is signed Int.\nTo store large number, try Double field instead", strings.ToTitle(f.Identifier)))
			}
			modifiedPayloads[f.Identifier] = _int
		} else {
			if newVal, ok := inputPayload[f.Identifier]; ok && newVal != nil {
				modifiedPayloads[f.Identifier] = newVal
			} else if val != nil {
				modifiedPayloads[f.Identifier] = val
			}
			oldPayload[f.Identifier] = val
		}
	}
	return modifiedPayloads, oldPayload, nil
}

func (s *GraphQLServer) runWebHook(event string, hook *models.Webhook, payload interface{}) error {
	id := utility.NewID()
	hookPost := models.WebhookPost{
		Id:      id,
		Event:   event,
		Model:   hook.Model,
		Payload: payload,
	}

	// send post request
	jsonStr, _ := json.Marshal(hookPost)
	req, err := http.NewRequest("POST", hook.URL, bytes.NewBuffer(jsonStr))
	if err != nil {
		utility.CaptureInternalServerError(err, nil)
		return err
	}
	//req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	// Add timeout to prevent goroutine leaks
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		utility.CaptureInternalServerError(err, nil)
		return err
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utility.CaptureInternalServerError(err, nil)
		return err
	}
	fmt.Println("response Body:", string(body))
	return nil
}

func (s *GraphQLServer) triggerFunction(ctx context.Context, f *models.ApitoFunction, cache *models.ApplicationCache, hook *models.Webhook, data map[string]interface{}) {
	// Create a new context that won't be canceled when the parent is canceled
	// but preserves all the values from the original context
	detachedCtx := context.WithoutCancel(ctx)
	paramCtx := context.WithValue(detachedCtx, "raw_payload", data)

	_, _, err := s.HandleApitoFunction(paramCtx, cache, f.Name, data)
	if err != nil {
		fmt.Println(err.Error())
		utility.CaptureInternalServerError(err, nil)
	}
	//return result, nil
}

func (s *GraphQLServer) triggerFunctionOLD(f *models.ApitoFunction, event string, hook *models.Webhook, data interface{}) {
	/*
		var cred *protobuff.ThirdPartyCredential
		if val, ok := s.PluginConfigurations["aws"]; ok {
			cred = val.Credentials
		} else {
			fmt.Println(errors.New("AWS Credentials are not Set"))
		}

		id := utility.NewID()
		hookPost := models.WebhookPost{
			Id:      id,
			Event:   event,
			Model:   hook.Model,
			Payload: data,
		}

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(f.ProviderConfig.Region),
			Credentials: credentials.NewStaticCredentials(cred.AccessKey, cred.SecretKey, ""),
		})
		if err != nil {
			fmt.Println(err)
		}
		_, err = sess.Config.Credentials.Get()
		if err != nil {
			fmt.Println(err)
		}

		payload, err := json.Marshal(hookPost)
		if err != nil {
			sentry.CaptureException(err)
			sentry.Flush(time.Second * 2)
		}

		svc := lambda.New(sess)
		input := &lambda.InvokeInput{
			FunctionName:   aws.String(f.ProviderConfig.RemoteFunctionName),
			Payload:        payload,
			InvocationType: aws.String("RequestResponse"),
			LogType:        aws.String("Tail"),
			//Qualifier:      aws.String("current"),
		}

		invokeResponse, err := svc.Invoke(input)
		if err != nil {
			sentry.CaptureException(err)
			sentry.Flush(time.Second * 2)
		}

		var result interface{}
		err = json.Unmarshal(invokeResponse.Payload, &result)
		if err != nil {
			sentry.CaptureException(err)
			sentry.Flush(time.Second * 2)
		}*/
}
