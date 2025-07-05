package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// BillingInfo represents billing information for display
type BillingInfo struct {
	Organization   string    `json:"organization"`
	Plan           string    `json:"plan"`
	HoursUsed      float64   `json:"hours_used"`
	HoursIncluded  float64   `json:"hours_included"`
	PercentUsed    int       `json:"percent_used"`
	InOverage      bool      `json:"in_overage"`
	ResetDate      time.Time `json:"reset_date"`
	EstimatedBill  float64   `json:"estimated_bill"`
	DailyUsage     string    `json:"daily_usage,omitempty"`
	ThrottleStatus string    `json:"throttle_status,omitempty"`
}

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "View billing and usage information",
	Long: `View your current billing plan, usage, and estimated charges for Orzbob Cloud.

This command shows:
- Your current subscription plan
- Hours used vs included in your plan
- Whether you're in overage
- When your usage resets
- Estimated charges for the current period`,
	RunE: runBilling,
}

var (
	billingJSON bool
)

func init() {
	cloudCmd.AddCommand(billingCmd)
	billingCmd.Flags().BoolVar(&billingJSON, "json", false, "Output as JSON")
}

func runBilling(cmd *cobra.Command, args []string) error {
	// Get auth token
	token, err := loadToken()
	if err != nil {
		return fmt.Errorf("not logged in. Run 'orz login' first")
	}

	// Call API to get billing info
	apiURL := getAPIURL()
	billingInfo, err := fetchBillingInfo(apiURL, token)
	if err != nil {
		return fmt.Errorf("failed to fetch billing information: %w", err)
	}

	// Output the results
	if billingJSON {
		return outputBillingJSON(billingInfo)
	}
	return outputBillingTable(billingInfo)
}

func fetchBillingInfo(apiURL, token string) (*BillingInfo, error) {
	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Create request
	req, err := http.NewRequest("GET", apiURL+"/v1/billing", nil)
	if err != nil {
		return nil, err
	}
	
	// Add auth header
	req.Header.Set("Authorization", "Bearer "+token)
	
	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var rawResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Convert to BillingInfo
	billingInfo := &BillingInfo{
		Organization:   getString(rawResp, "organization"),
		Plan:           getString(rawResp, "plan"),
		HoursUsed:      getFloat64(rawResp, "hours_used"),
		HoursIncluded:  getFloat64(rawResp, "hours_included"),
		PercentUsed:    int(getFloat64(rawResp, "percent_used")),
		InOverage:      getBool(rawResp, "in_overage"),
		EstimatedBill:  getFloat64(rawResp, "estimated_bill"),
		DailyUsage:     getString(rawResp, "daily_usage"),
		ThrottleStatus: getString(rawResp, "throttle_status"),
	}
	
	// Parse reset date
	if resetStr := getString(rawResp, "reset_date"); resetStr != "" {
		if t, err := time.Parse(time.RFC3339, resetStr); err == nil {
			billingInfo.ResetDate = t
		} else {
			billingInfo.ResetDate = time.Now().AddDate(0, 1, 0) // Default to 1 month
		}
	}
	
	return billingInfo, nil
}

// Helper functions for safe type conversion
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func outputBillingJSON(info *BillingInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(info)
}

func outputBillingTable(info *BillingInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "BILLING INFORMATION")
	fmt.Fprintln(w, "==================")
	fmt.Fprintf(w, "Organization:\t%s\n", info.Organization)
	fmt.Fprintf(w, "Plan:\t%s\n", info.Plan)
	fmt.Fprintln(w)
	
	fmt.Fprintln(w, "USAGE")
	fmt.Fprintln(w, "-----")
	fmt.Fprintf(w, "Hours Used:\t%.1f / %.0f (%.0f%%)\n", info.HoursUsed, info.HoursIncluded, float64(info.PercentUsed))
	
	// Show usage bar
	fmt.Fprint(w, "Progress:\t")
	printUsageBar(w, info.PercentUsed)
	fmt.Fprintln(w)
	
	if info.InOverage {
		fmt.Fprintln(w, "Status:\t⚠️  IN OVERAGE - Additional charges apply")
	} else {
		fmt.Fprintln(w, "Status:\t✅ Within included hours")
	}
	
	if info.DailyUsage != "" {
		fmt.Fprintf(w, "Today's Usage:\t%s\n", info.DailyUsage)
	}
	
	if info.ThrottleStatus != "" {
		fmt.Fprintf(w, "Throttle Status:\t%s\n", info.ThrottleStatus)
	}
	
	fmt.Fprintln(w)
	fmt.Fprintln(w, "BILLING")
	fmt.Fprintln(w, "-------")
	fmt.Fprintf(w, "Estimated Bill:\t$%.2f\n", info.EstimatedBill)
	fmt.Fprintf(w, "Resets On:\t%s (%d days)\n", 
		info.ResetDate.Format("Jan 2, 2006"),
		int(time.Until(info.ResetDate).Hours()/24))
	
	fmt.Fprintln(w)
	fmt.Fprintln(w, "For more details, visit: https://orzbob.cloud/billing")
	
	return nil
}

func printUsageBar(w *tabwriter.Writer, percent int) {
	const barWidth = 30
	filled := (percent * barWidth) / 100
	if filled > barWidth {
		filled = barWidth
	}
	
	fmt.Fprint(w, "[")
	for i := 0; i < barWidth; i++ {
		if i < filled {
			fmt.Fprint(w, "█")
		} else {
			fmt.Fprint(w, "░")
		}
	}
	fmt.Fprint(w, "]")
}

// Helper function to get API URL (reuse existing logic)
func getAPIURL() string {
	if url := os.Getenv("ORZBOB_API_URL"); url != "" {
		return url
	}
	return "https://api.orzbob.cloud"
}