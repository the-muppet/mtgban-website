package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"google.golang.org/api/secretmanager/v1"
)

func initSecretClient(projectId string) error {
	ctx := context.Background()
	var err error
	SecretsClient, err = secretmanager.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	return nil
}

func downloadSecret(projectID, secretID string) ([]byte, error) {

	accessReq := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secretID)

	result, err := SecretsClient.Projects.Secrets.Versions.Access(accessReq).Do()
	if err != nil {
		return nil, err
	}

	decodedData, err := base64.StdEncoding.DecodeString(result.Payload.Data)
	if err != nil {
		return nil, err
	}

	return decodedData, nil
}

func addSecretVersion(projectID, secretID, payload string) error {

	parent := fmt.Sprintf("projects/%s/secrets/%s", projectID, secretID)

	encodedPayload := base64.StdEncoding.EncodeToString([]byte(payload))

	secretPayload := &secretmanager.AddSecretVersionRequest{
		Payload: &secretmanager.SecretPayload{
			Data: encodedPayload,
		},
	}

	_, err := SecretsClient.Projects.Secrets.AddVersion(parent, secretPayload).Do()
	if err != nil {
		log.Printf("failed to add secret version: %v", err)
		return err
	}

	log.Printf("new version of %s added", secretID)
	return nil
}
