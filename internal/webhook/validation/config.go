// TODO:
/*
	1) Add necessary imports
	2) Config fields -> to be added. Based on the config for the etcdcomponents. Trim unnecessary fields for now
	3) Flags to be added for the config for the InitFromFlags function
	4) Check if the Helper funcitons as defined in the ../etcdcomponents/config.go are needed
	5) GOTO register.go
*/
// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package validation

import (
	"fmt"

	flag "github.com/spf13/pflag"
)

const ( // TODO Decide the required flags (if any)
	enableValidationWebhookFlagName = "enable-validation-webhook"
	defaultEnableValidationWebhook  = true
)

type Config struct { // only enabled for now. TOBEADDED: Sevice account List
	Enabled bool
}

func InitFromFlags(fs *flag.FlagSet, cfg *Config) {
	fmt.Println("Reached HERE") // getting reached
	fs.BoolVar(&cfg.Enabled, enableValidationWebhookFlagName, defaultEnableValidationWebhook,
		"Enable the Validation Webhook to ensure resource changes conform to desired criteria.")
}
