// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"
	"github.com/gardener/etcd-druid/internal/webhook/etcdcomponents"
	"github.com/gardener/etcd-druid/internal/webhook/validation"

	"golang.org/x/exp/slog"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Register registers all etcd-druid webhooks with the controller manager.
func Register(mgr ctrl.Manager, config *Config) error {
	// Add Etcd Components webhook to the manager
	if config.EtcdComponents.Enabled {
		etcdComponentsWebhook, err := etcdcomponents.NewHandler(
			mgr,
			config.EtcdComponents,
		)
		if err != nil {
			return err
		}
		slog.Info("Registering EtcdComponents Webhook with manager")
		// return etcdComponentsWebhook.RegisterWithManager(mgr)
		if err := etcdComponentsWebhook.RegisterWithManager(mgr); err != nil {
			return err
		}
	}
	fmt.Println("REGISTERED COMPONENTS")
	// Register Validation webhook with the manager.
	if config.Validation.Enabled {
		validationWebhook, err := validation.NewHandler(
			mgr,
			config.Validation,
		)
		if err != nil {
			fmt.Println("The error is here, 40, register_manager")
			return err
		}
		slog.Info("Registering Validation Webhook with manager")
		// return validationWebhook.RegisterWithManager(mgr)
		if err := validationWebhook.RegisterWithManager(mgr); err != nil {
			return err
		}
		fmt.Println("REGISTERED VALIDATE")
	}
	return nil
}
