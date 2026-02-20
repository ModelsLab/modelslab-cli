package cmd

import (
	"fmt"

	"github.com/ModelsLab/modelslab-cli/internal/api"
	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

const stripePublishableKey = "pk_live_51JfPKxSDo1BGXG2xQS6wNZlCIoNBJFBINvJXrzKFYHiJW6wOfInnLRKuKSbPcJj7QEBd9bVzLXIiXW6TW2nT0FJF006u7qX9kH"

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Manage billing and payment methods",
}

var billingOverviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "View billing overview",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/overview", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for _, key := range []string{"wallet_balance", "active_subscription", "total_charges", "total_deposits"} {
				if v, ok := data[key]; ok && v != nil {
					pairs = append(pairs, [2]string{key, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var billingPaymentMethodsCmd = &cobra.Command{
	Use:   "payment-methods",
	Short: "List payment methods",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/payment-methods", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "BRAND", "LAST4", "EXPIRY", "DEFAULT"}
			rows := [][]string{}
			for _, item := range data {
				if pm, ok := item.(map[string]interface{}); ok {
					isDefault := ""
					if d, ok := pm["is_default"].(bool); ok && d {
						isDefault = "Yes"
					}
					rows = append(rows, []string{
						fmt.Sprintf("%v", pm["id"]),
						fmt.Sprintf("%v", pm["brand"]),
						fmt.Sprintf("%v", pm["last4"]),
						fmt.Sprintf("%v/%v", pm["exp_month"], pm["exp_year"]),
						isDefault,
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var billingAddPMCmd = &cobra.Command{
	Use:   "add-payment-method",
	Short: "Add a payment method",
	RunE: func(cmd *cobra.Command, args []string) error {
		pmID, _ := cmd.Flags().GetString("payment-method")
		setDefault, _ := cmd.Flags().GetBool("default")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		// If no pm_id provided, tokenize card via Stripe
		if pmID == "" {
			cardNumber, _ := cmd.Flags().GetString("card-number")
			expMonth, _ := cmd.Flags().GetString("exp-month")
			expYear, _ := cmd.Flags().GetString("exp-year")
			cvc, _ := cmd.Flags().GetString("cvc")

			if cardNumber == "" {
				return fmt.Errorf("provide --payment-method or card details (--card-number, --exp-month, --exp-year, --cvc)")
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Tokenizing card via Stripe...")
			client := getClient()
			stripeResult, err := client.DoStripe(stripePublishableKey, map[string]string{
				"type":             "card",
				"card[number]":     cardNumber,
				"card[exp_month]":  expMonth,
				"card[exp_year]":   expYear,
				"card[cvc]":        cvc,
			})
			if err != nil {
				return err
			}

			id, ok := stripeResult["id"].(string)
			if !ok {
				return fmt.Errorf("could not get payment method ID from Stripe")
			}
			pmID = id
			fmt.Fprintln(cmd.ErrOrStderr(), "done")
		}

		body := map[string]interface{}{
			"payment_method_id": pmID,
			"set_default":      setDefault,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/billing/payment-methods", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Payment method %s added.", pmID))
		})
		return nil
	},
}

var billingSetDefaultCmd = &cobra.Command{
	Use:   "set-default",
	Short: "Set default payment method",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("PUT", "/billing/payment-methods/"+id+"/default", nil, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Default payment method set to " + id)
		})
		return nil
	},
}

var billingRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a payment method",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("DELETE", "/billing/payment-methods/"+id, nil, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Payment method " + id + " removed.")
		})
		return nil
	},
}

var billingInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get billing info",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/info", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for k, v := range data {
				if v != nil {
					pairs = append(pairs, [2]string{k, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var billingUpdateInfoCmd = &cobra.Command{
	Use:   "update-info",
	Short: "Update billing info",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("name"); v != "" {
			body["name"] = v
		}
		if v, _ := cmd.Flags().GetString("address"); v != "" {
			body["address"] = v
		}
		if v, _ := cmd.Flags().GetString("city"); v != "" {
			body["city"] = v
		}
		if v, _ := cmd.Flags().GetString("country"); v != "" {
			body["country"] = v
		}
		if v, _ := cmd.Flags().GetString("tax-id"); v != "" {
			body["tax_id"] = v
		}
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("PUT", "/billing/info", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Billing info updated.")
		})
		return nil
	},
}

var billingInvoicesCmd = &cobra.Command{
	Use:   "invoices",
	Short: "List invoices",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/invoices", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "AMOUNT", "STATUS", "DATE"}
			rows := [][]string{}
			for _, item := range data {
				if inv, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", inv["id"]),
						fmt.Sprintf("$%v", inv["amount"]),
						fmt.Sprintf("%v", inv["status"]),
						fmt.Sprintf("%v", inv["created_at"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var billingInvoiceDetailCmd = &cobra.Command{
	Use:   "invoice",
	Short: "View invoice details",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/invoices/"+id, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintJSON(result)
		})
		return nil
	},
}

var billingStripeConfigCmd = &cobra.Command{
	Use:   "stripe-config",
	Short: "Get Stripe publishable key for client-side card tokenization",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/stripe-config", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data := extractData(result)
			pairs := [][2]string{}
			if pk, ok := data["publishable_key"].(string); ok {
				pairs = append(pairs, [2]string{"publishable_key", pk})
			}
			if instr, ok := data["instructions"].(string); ok {
				pairs = append(pairs, [2]string{"instructions", instr})
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var billingPaymentLinkCmd = &cobra.Command{
	Use:   "payment-link",
	Short: "Create a Stripe-hosted payment URL for human-assisted payments",
	Long: `Create a Stripe Checkout payment link that a human can open in a browser.
Used for the human-assisted payment flow where the CLI cannot tokenize cards directly.

The success_url and cancel_url are controlled by ModelsLab and cannot be overridden.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		purpose, _ := cmd.Flags().GetString("purpose")
		amount, _ := cmd.Flags().GetFloat64("amount")
		planID, _ := cmd.Flags().GetInt("plan-id")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"purpose": purpose,
		}
		if purpose == "fund" {
			if amount <= 0 {
				return fmt.Errorf("--amount is required when purpose is 'fund'")
			}
			body["amount"] = amount
		} else if purpose == "subscribe" {
			if planID <= 0 {
				return fmt.Errorf("--plan-id is required when purpose is 'subscribe'")
			}
			body["plan_id"] = planID
		} else {
			return fmt.Errorf("--purpose must be 'fund' or 'subscribe'")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/billing/payment-link", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data := extractData(result)
			if url, ok := data["payment_url"].(string); ok {
				fmt.Println("Payment URL:", url)
				fmt.Println("Open this URL in a browser to complete payment.")
			}
			if sessionID, ok := data["session_id"].(string); ok {
				fmt.Printf("Session ID: %s\n", sessionID)
			}
			if expiresAt, ok := data["expires_at"].(string); ok {
				fmt.Printf("Expires at: %s\n", expiresAt)
			}
		})
		return nil
	},
}

var billingSetupIntentCmd = &cobra.Command{
	Use:   "setup-intent",
	Short: "Create a Stripe SetupIntent for saving payment methods",
	Long: `Create a Stripe SetupIntent to save a payment method for future use.
The returned client_secret can be used with Stripe.js or the Stripe API
to confirm the setup and attach the payment method to the customer.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/billing/setup-intent", nil, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data := extractData(result)
			pairs := [][2]string{}
			for _, key := range []string{"setup_intent_id", "client_secret", "status"} {
				if v, ok := data[key]; ok && v != nil {
					pairs = append(pairs, [2]string{key, fmt.Sprintf("%v", v)})
				}
			}
			if instr, ok := data["instructions"].(string); ok {
				pairs = append(pairs, [2]string{"instructions", instr})
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var billingInvoicePDFCmd = &cobra.Command{
	Use:   "invoice-pdf",
	Short: "Download invoice PDF",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/billing/invoices/"+id+"/pdf", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if url, ok := result["url"].(string); ok {
				fmt.Println("PDF URL:", url)
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

func init() {
	_ = api.ExitSuccess // suppress unused import

	billingAddPMCmd.Flags().String("payment-method", "", "Stripe payment method ID")
	billingAddPMCmd.Flags().String("card-number", "", "Card number")
	billingAddPMCmd.Flags().String("exp-month", "", "Expiry month (MM)")
	billingAddPMCmd.Flags().String("exp-year", "", "Expiry year (YYYY)")
	billingAddPMCmd.Flags().String("cvc", "", "CVC")
	billingAddPMCmd.Flags().Bool("default", false, "Set as default")
	billingAddPMCmd.Flags().String("idempotency-key", "", "Idempotency key")

	billingSetDefaultCmd.Flags().String("id", "", "Payment method ID")
	billingSetDefaultCmd.Flags().String("idempotency-key", "", "Idempotency key")

	billingRemoveCmd.Flags().String("id", "", "Payment method ID")
	billingRemoveCmd.Flags().String("idempotency-key", "", "Idempotency key")

	billingUpdateInfoCmd.Flags().String("name", "", "Billing name")
	billingUpdateInfoCmd.Flags().String("address", "", "Address")
	billingUpdateInfoCmd.Flags().String("city", "", "City")
	billingUpdateInfoCmd.Flags().String("country", "", "Country")
	billingUpdateInfoCmd.Flags().String("tax-id", "", "Tax ID")
	billingUpdateInfoCmd.Flags().String("idempotency-key", "", "Idempotency key")

	billingInvoiceDetailCmd.Flags().String("id", "", "Invoice ID")
	billingInvoicePDFCmd.Flags().String("id", "", "Invoice ID")

	billingPaymentLinkCmd.Flags().String("purpose", "", "Payment purpose: 'fund' or 'subscribe'")
	billingPaymentLinkCmd.Flags().Float64("amount", 0, "Amount in USD (required when purpose is 'fund')")
	billingPaymentLinkCmd.Flags().Int("plan-id", 0, "Plan ID (required when purpose is 'subscribe')")
	billingPaymentLinkCmd.Flags().String("idempotency-key", "", "Idempotency key")
	billingPaymentLinkCmd.MarkFlagRequired("purpose")

	billingSetupIntentCmd.Flags().String("idempotency-key", "", "Idempotency key")

	billingCmd.AddCommand(billingOverviewCmd)
	billingCmd.AddCommand(billingPaymentMethodsCmd)
	billingCmd.AddCommand(billingAddPMCmd)
	billingCmd.AddCommand(billingSetDefaultCmd)
	billingCmd.AddCommand(billingRemoveCmd)
	billingCmd.AddCommand(billingInfoCmd)
	billingCmd.AddCommand(billingUpdateInfoCmd)
	billingCmd.AddCommand(billingInvoicesCmd)
	billingCmd.AddCommand(billingInvoiceDetailCmd)
	billingCmd.AddCommand(billingInvoicePDFCmd)
	billingCmd.AddCommand(billingStripeConfigCmd)
	billingCmd.AddCommand(billingPaymentLinkCmd)
	billingCmd.AddCommand(billingSetupIntentCmd)
}
