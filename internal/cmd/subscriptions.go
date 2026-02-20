package cmd

import (
	"fmt"

	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var subscriptionsCmd = &cobra.Command{
	Use:     "subscriptions",
	Aliases: []string{"subs"},
	Short:   "Manage subscriptions",
}

var subsPlansCmd = &cobra.Command{
	Use:   "plans",
	Short: "List available plans",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/subscriptions/plans", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "NAME", "PRICE", "INTERVAL", "FEATURES"}
			rows := [][]string{}
			for _, item := range data {
				if plan, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", plan["id"]),
						fmt.Sprintf("%v", plan["name"]),
						fmt.Sprintf("$%v", plan["price"]),
						fmt.Sprintf("%v", plan["interval"]),
						truncate(fmt.Sprintf("%v", plan["description"]), 40),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var subsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/subscriptions", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "PLAN", "STATUS", "NEXT BILLING"}
			rows := [][]string{}
			for _, item := range data {
				if sub, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", sub["id"]),
						fmt.Sprintf("%v", sub["plan_name"]),
						fmt.Sprintf("%v", sub["status"]),
						fmt.Sprintf("%v", sub["next_billing_date"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var subsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a subscription",
	RunE: func(cmd *cobra.Command, args []string) error {
		planID, _ := cmd.Flags().GetInt("plan-id")
		pmID, _ := cmd.Flags().GetString("payment-method")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		// Tokenize card if needed
		if pmID == "" {
			cardNumber, _ := cmd.Flags().GetString("card-number")
			if cardNumber != "" {
				expMonth, _ := cmd.Flags().GetString("exp-month")
				expYear, _ := cmd.Flags().GetString("exp-year")
				cvc, _ := cmd.Flags().GetString("cvc")

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
		}

		if dryRun {
			fmt.Println("Would create subscription:")
			fmt.Printf("  Plan ID: %d\n", planID)
			if pmID != "" {
				fmt.Printf("  Payment method: %s\n", pmID)
				fmt.Println("  Flow: Headless (direct charge, no redirect)")
			} else {
				fmt.Println("  Flow: Stripe Checkout (redirect)")
			}
			fmt.Println("  Use --yes to proceed")
			return nil
		}

		body := map[string]interface{}{
			"plan_id": planID,
		}
		if pmID != "" {
			body["payment_method_id"] = pmID
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/subscriptions", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if data, ok := result["data"].(map[string]interface{}); ok {
				if url, ok := data["checkout_url"].(string); ok && url != "" {
					fmt.Println("Checkout URL:", url)
					fmt.Println("Open this URL to complete payment.")
					return
				}
			}
			output.PrintSuccess("Subscription created!")
		})
		return nil
	},
}

var subsConfirmCheckoutCmd = &cobra.Command{
	Use:   "confirm-checkout",
	Short: "Confirm Stripe Checkout session",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"session_id": sessionID,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/subscriptions/confirm-checkout", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Subscription checkout confirmed.")
		})
		return nil
	},
}

var subsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check subscription status",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/subscriptions/"+id+"/status", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for _, key := range []string{"status", "plan_name", "next_billing_date", "amount"} {
				if v, ok := data[key]; ok && v != nil {
					pairs = append(pairs, [2]string{key, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

var subsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update subscription plan",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		newPlanID, _ := cmd.Flags().GetInt("new-plan-id")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"plan_id": newPlanID,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("PUT", "/subscriptions/"+id, body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Subscription updated.")
		})
		return nil
	},
}

func makeSubsActionCmd(action, short, httpMethod string) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

			client := getClient()
			var result map[string]interface{}
			err := client.DoControlPlaneIdempotent(httpMethod, "/subscriptions/"+id+"/"+action, nil, &result, idempotencyKey)
			if err != nil {
				return err
			}

			outputResult(result, func() {
				output.PrintSuccess(fmt.Sprintf("Subscription %s.", action))
			})
			return nil
		},
	}
}

var subsPauseCmd = makeSubsActionCmd("pause", "Pause subscription", "POST")
var subsResumeCmd = makeSubsActionCmd("resume", "Resume subscription", "POST")
var subsResetCycleCmd = makeSubsActionCmd("reset-cycle", "Reset billing cycle", "POST")

var subsChargeAmountCmd = &cobra.Command{
	Use:   "charge-amount",
	Short: "Set charge amount",
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, _ := cmd.Flags().GetFloat64("amount")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"amount": amount,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/subscriptions/charge-amount", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Charge amount set to $%.2f", amount))
		})
		return nil
	},
}

func init() {
	subsCreateCmd.Flags().Int("plan-id", 0, "Plan ID")
	subsCreateCmd.Flags().String("payment-method", "", "Payment method ID")
	subsCreateCmd.Flags().String("card-number", "", "Card number")
	subsCreateCmd.Flags().String("exp-month", "", "Expiry month")
	subsCreateCmd.Flags().String("exp-year", "", "Expiry year")
	subsCreateCmd.Flags().String("cvc", "", "CVC")
	subsCreateCmd.Flags().Bool("dry-run", false, "Preview without creating")
	subsCreateCmd.Flags().String("idempotency-key", "", "Idempotency key")
	subsCreateCmd.MarkFlagRequired("plan-id")

	subsConfirmCheckoutCmd.Flags().String("session-id", "", "Stripe session ID")
	subsConfirmCheckoutCmd.Flags().String("idempotency-key", "", "Idempotency key")
	subsConfirmCheckoutCmd.MarkFlagRequired("session-id")

	subsStatusCmd.Flags().String("id", "", "Subscription ID")

	subsUpdateCmd.Flags().String("id", "", "Subscription ID")
	subsUpdateCmd.Flags().Int("new-plan-id", 0, "New plan ID")
	subsUpdateCmd.Flags().String("idempotency-key", "", "Idempotency key")
	subsUpdateCmd.MarkFlagRequired("id")
	subsUpdateCmd.MarkFlagRequired("new-plan-id")

	for _, c := range []*cobra.Command{subsPauseCmd, subsResumeCmd, subsResetCycleCmd} {
		c.Flags().String("id", "", "Subscription ID")
		c.Flags().String("idempotency-key", "", "Idempotency key")
		c.MarkFlagRequired("id")
	}

	subsChargeAmountCmd.Flags().Float64("amount", 0, "Charge amount in USD")
	subsChargeAmountCmd.Flags().String("idempotency-key", "", "Idempotency key")
	subsChargeAmountCmd.MarkFlagRequired("amount")

	subscriptionsCmd.AddCommand(subsPlansCmd)
	subscriptionsCmd.AddCommand(subsListCmd)
	subscriptionsCmd.AddCommand(subsCreateCmd)
	subscriptionsCmd.AddCommand(subsConfirmCheckoutCmd)
	subscriptionsCmd.AddCommand(subsStatusCmd)
	subscriptionsCmd.AddCommand(subsUpdateCmd)
	subscriptionsCmd.AddCommand(subsPauseCmd)
	subscriptionsCmd.AddCommand(subsResumeCmd)
	subscriptionsCmd.AddCommand(subsResetCycleCmd)
	subscriptionsCmd.AddCommand(subsChargeAmountCmd)
}
