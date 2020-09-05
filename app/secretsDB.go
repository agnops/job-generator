package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"strings"
)

func getWebhookSecret(secretName string) string {
	secret, err := secretsClient.Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Println(err.Error())
	}
	if secret == nil || secret.GetName() != secretName {
		webhookSecret, _ := randomHex(20)
		createWebhookSecret(secretName, webhookSecret)
		return webhookSecret
	}
	return string(secret.Data["WebhookSecret"])
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func createWebhookSecret(secretName string, webhookSecret string) {
	secretSpec := apiv1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"AgnOps":      "WebhookSecret",
			},
		},
		Data: map[string][]byte{
			"WebhookSecret":   []byte(webhookSecret),
		},
		Type: "Opaque",
	}

	secretName = secretSpec.ObjectMeta.Name

	_, err := secretsClient.Create(context.TODO(), &secretSpec, metav1.CreateOptions{})
	if err != nil {
		log.Println(err.Error())
	}
	log.Printf("Created secret %s\n", secretName)
}

func GetUserOrOrganizationToken(scmProvider string, userOrg string) (string, error) {
	secretName := "agnops-" + strings.ToLower(scmProvider) + "-" + strings.ToLower(userOrg)
	secret, err := secretsClient.Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Println(err.Error())
		return "", err
	}
	return string(secret.Data["OAuth2Token"]), nil
}

func GetOwnerOrRepositoryName(htmlUrl string) string {
	i := strings.LastIndex(htmlUrl, "/")
	var orgOrUserName string = htmlUrl

	return orgOrUserName[i+1:]
}