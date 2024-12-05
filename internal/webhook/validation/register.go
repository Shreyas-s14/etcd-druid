/*
TODO:
	1) add path and handlername for /webhooks/validation. Name: "etcd-validation-webhook"
	2) RegisterWithManager
	GOTO:
	1) ../register.go -> register handler for second webhook. Handle multiple at once
	2) handler.go -> check how its done in the etcdcomponents webhook. Common handler functions to be added in miscellaneous.go in utils

*/
// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package validation

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	handlerName = "etcd-validation-webhook"
	webhookPath = "/webhooks/validation" // alternative: /webhooks/etcd/validation
)

func (h *Handler) RegisterWithManager(mgr manager.Manager) error {
	webhook := &admission.Webhook{
		Handler:      h, // TODO: IMplement Handle in handler.go
		RecoverPanic: true,
	}
	mgr.GetWebhookServer().Register(webhookPath, webhook)
	fmt.Println("REGISTERED THE WEBHOOK WITH THE MANAGER")
	return nil
}
