package cmd

import (
	"fmt"

	"github.com/ModelsLab/modelslab-cli/internal/output"
	"github.com/spf13/cobra"
)

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage wallet balance and funding",
}

var walletBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Check wallet balance",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/wallet/balance", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data := extractData(result)
			// Handle nested wallet object: data.wallet.balance
			if wallet, ok := data["wallet"].(map[string]interface{}); ok {
				fmt.Printf("Wallet balance: $%v\n", wallet["balance"])
				if currency, ok := wallet["currency"]; ok && currency != nil {
					fmt.Printf("Currency: %v\n", currency)
				}
			} else if balance, ok := data["balance"]; ok {
				fmt.Printf("Wallet balance: $%v\n", balance)
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

var walletFundCmd = &cobra.Command{
	Use:   "fund",
	Short: "Add funds to wallet",
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, _ := cmd.Flags().GetFloat64("amount")
		pmID, _ := cmd.Flags().GetString("payment-method")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		if amount < 10 {
			return fmt.Errorf("minimum funding amount is $10")
		}

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

		body := map[string]interface{}{
			"amount": amount,
		}
		if pmID != "" {
			body["payment_method_id"] = pmID
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/wallet/fund", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Funded $%.2f to wallet.", amount))
			if data, ok := result["data"].(map[string]interface{}); ok {
				if balance, ok := data["balance"]; ok {
					fmt.Printf("New balance: $%v\n", balance)
				}
			}
		})
		return nil
	},
}

var walletConfirmCheckoutCmd = &cobra.Command{
	Use:   "confirm-checkout",
	Short: "Confirm a Stripe Checkout session",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"session_id": sessionID,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/wallet/confirm-checkout", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Checkout confirmed.")
		})
		return nil
	},
}

var walletTransactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "View wallet transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/wallet/transactions", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].([]interface{})
			if !ok {
				output.PrintJSON(result)
				return
			}
			headers := []string{"ID", "TYPE", "AMOUNT", "DESCRIPTION", "DATE"}
			rows := [][]string{}
			for _, item := range data {
				if tx, ok := item.(map[string]interface{}); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", tx["id"]),
						fmt.Sprintf("%v", tx["type"]),
						fmt.Sprintf("$%v", tx["amount"]),
						fmt.Sprintf("%v", tx["description"]),
						fmt.Sprintf("%v", tx["created_at"]),
					})
				}
			}
			output.PrintTable(headers, rows)
		})
		return nil
	},
}

var walletAutoFundingCmd = &cobra.Command{
	Use:   "auto-funding",
	Short: "Enable auto-recharge",
	RunE: func(cmd *cobra.Command, args []string) error {
		threshold, _ := cmd.Flags().GetFloat64("threshold")
		amount, _ := cmd.Flags().GetFloat64("amount")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"threshold": threshold,
			"amount":    amount,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("PUT", "/wallet/auto-funding", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Auto-funding enabled: recharge $%.2f when balance drops below $%.2f", amount, threshold))
		})
		return nil
	},
}

var walletDisableAutoFundingCmd = &cobra.Command{
	Use:   "disable-auto-funding",
	Short: "Disable auto-recharge",
	RunE: func(cmd *cobra.Command, args []string) error {
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("DELETE", "/wallet/auto-funding", nil, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Auto-funding disabled.")
		})
		return nil
	},
}

var walletWithdrawCmd = &cobra.Command{
	Use:   "withdraw",
	Short: "Withdraw funds (resellers)",
	RunE: func(cmd *cobra.Command, args []string) error {
		amount, _ := cmd.Flags().GetFloat64("amount")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"amount": amount,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/wallet/withdraw", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess(fmt.Sprintf("Withdrawal of $%.2f initiated.", amount))
		})
		return nil
	},
}

var walletValidateCouponCmd = &cobra.Command{
	Use:   "validate-coupon",
	Short: "Validate a coupon code",
	RunE: func(cmd *cobra.Command, args []string) error {
		code, _ := cmd.Flags().GetString("code")

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/wallet/coupons/validate?code="+code, nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintJSON(result)
		})
		return nil
	},
}

var walletRedeemCouponCmd = &cobra.Command{
	Use:   "redeem-coupon",
	Short: "Redeem a coupon code",
	RunE: func(cmd *cobra.Command, args []string) error {
		code, _ := cmd.Flags().GetString("code")
		idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

		body := map[string]interface{}{
			"code": code,
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlaneIdempotent("POST", "/wallet/coupons/redeem", body, &result, idempotencyKey)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			output.PrintSuccess("Coupon redeemed!")
		})
		return nil
	},
}

var walletPaymentStatusCmd = &cobra.Command{
	Use:   "payment-status",
	Short: "Check payment intent status",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoControlPlane("GET", "/payments/"+id+"/status", nil, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			data, ok := result["data"].(map[string]interface{})
			if !ok {
				data = result
			}
			pairs := [][2]string{}
			for _, key := range []string{"status", "amount", "currency"} {
				if v, ok := data[key]; ok && v != nil {
					pairs = append(pairs, [2]string{key, fmt.Sprintf("%v", v)})
				}
			}
			output.PrintKeyValue(pairs)
		})
		return nil
	},
}

func init() {
	walletFundCmd.Flags().Float64("amount", 0, "Amount in USD (min $10)")
	walletFundCmd.Flags().String("payment-method", "", "Payment method ID")
	walletFundCmd.Flags().String("card-number", "", "Card number")
	walletFundCmd.Flags().String("exp-month", "", "Expiry month")
	walletFundCmd.Flags().String("exp-year", "", "Expiry year")
	walletFundCmd.Flags().String("cvc", "", "CVC")
	walletFundCmd.Flags().String("idempotency-key", "", "Idempotency key")
	walletFundCmd.MarkFlagRequired("amount")

	walletConfirmCheckoutCmd.Flags().String("session-id", "", "Stripe session ID")
	walletConfirmCheckoutCmd.Flags().String("idempotency-key", "", "Idempotency key")
	walletConfirmCheckoutCmd.MarkFlagRequired("session-id")

	walletAutoFundingCmd.Flags().Float64("threshold", 0, "Balance threshold")
	walletAutoFundingCmd.Flags().Float64("amount", 0, "Recharge amount")
	walletAutoFundingCmd.Flags().String("idempotency-key", "", "Idempotency key")

	walletDisableAutoFundingCmd.Flags().String("idempotency-key", "", "Idempotency key")

	walletWithdrawCmd.Flags().Float64("amount", 0, "Withdrawal amount")
	walletWithdrawCmd.Flags().String("idempotency-key", "", "Idempotency key")
	walletWithdrawCmd.MarkFlagRequired("amount")

	walletValidateCouponCmd.Flags().String("code", "", "Coupon code")
	walletValidateCouponCmd.MarkFlagRequired("code")

	walletRedeemCouponCmd.Flags().String("code", "", "Coupon code")
	walletRedeemCouponCmd.Flags().String("idempotency-key", "", "Idempotency key")
	walletRedeemCouponCmd.MarkFlagRequired("code")

	walletPaymentStatusCmd.Flags().String("id", "", "Payment intent ID")

	walletCmd.AddCommand(walletBalanceCmd)
	walletCmd.AddCommand(walletFundCmd)
	walletCmd.AddCommand(walletConfirmCheckoutCmd)
	walletCmd.AddCommand(walletTransactionsCmd)
	walletCmd.AddCommand(walletAutoFundingCmd)
	walletCmd.AddCommand(walletDisableAutoFundingCmd)
	walletCmd.AddCommand(walletWithdrawCmd)
	walletCmd.AddCommand(walletValidateCouponCmd)
	walletCmd.AddCommand(walletRedeemCouponCmd)
	walletCmd.AddCommand(walletPaymentStatusCmd)
}
