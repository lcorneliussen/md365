package cmd

import (
	"time"

	"github.com/larsc/md365/internal/cal"
	"github.com/spf13/cobra"
)

var (
	calAccount   string
	calFrom      string
	calTo        string
	calSearch    string
	calSubject   string
	calStart     string
	calEnd       string
	calLocation  string
	calBody      string
	calID        string
	calFile      string
	calAttendees []string
	calForce     bool
)

// calCmd represents the cal command
var calCmd = &cobra.Command{
	Use:   "cal",
	Short: "Calendar commands",
	Long:  `Manage calendar events.`,
}

// calListCmd represents the cal list command
var calListCmd = &cobra.Command{
	Use:   "list",
	Short: "List calendar events",
	Long:  `List calendar events from local Markdown files.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Parse dates
		var fromDate, toDate time.Time
		var err error

		if calFrom != "" {
			fromDate, err = time.Parse("2006-01-02", calFrom)
			if err != nil {
				fatal(err)
			}
		} else {
			fromDate = time.Now()
		}

		if calTo != "" {
			toDate, err = time.Parse("2006-01-02", calTo)
			if err != nil {
				fatal(err)
			}
			// Set to end of day
			toDate = toDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		} else {
			toDate = time.Now().AddDate(0, 0, 14).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}

		if err := cal.List(cfg, fromDate, toDate, calSearch, calAccount); err != nil {
			fatal(err)
		}
	},
}

// calCreateCmd represents the cal create command
var calCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create calendar event",
	Long:  `Create a new calendar event via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if calAccount == "" || calSubject == "" || calStart == "" || calEnd == "" {
			fatal(cmd.Help())
			return
		}

		if err := cal.Create(cfg, calAccount, calSubject, calStart, calEnd, calLocation, calBody, calAttendees, calForce); err != nil {
			fatal(err)
		}
	},
}

// calDeleteCmd represents the cal delete command
var calDeleteCmd = &cobra.Command{
	Use:   "delete [file]",
	Short: "Delete calendar event",
	Long:  `Delete a calendar event via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if file path is provided as argument
		if len(args) > 0 {
			calFile = args[0]
		}

		if err := cal.Delete(cfg, calAccount, calID, calFile); err != nil {
			fatal(err)
		}
	},
}

func init() {
	// cal list
	calListCmd.Flags().StringVar(&calFrom, "from", "", "Start date (YYYY-MM-DD)")
	calListCmd.Flags().StringVar(&calTo, "to", "", "End date (YYYY-MM-DD)")
	calListCmd.Flags().StringVar(&calSearch, "search", "", "Search query")
	calListCmd.Flags().StringVar(&calAccount, "account", "", "Filter by account")

	// cal create
	calCreateCmd.Flags().StringVar(&calAccount, "account", "", "Account (required)")
	calCreateCmd.Flags().StringVar(&calSubject, "subject", "", "Event subject (required)")
	calCreateCmd.Flags().StringVar(&calStart, "start", "", "Start date/time (required)")
	calCreateCmd.Flags().StringVar(&calEnd, "end", "", "End date/time (required)")
	calCreateCmd.Flags().StringVar(&calLocation, "location", "", "Location")
	calCreateCmd.Flags().StringVar(&calBody, "body", "", "Body text")
	calCreateCmd.Flags().StringSliceVar(&calAttendees, "attendees", []string{}, "Attendee emails (comma-separated)")
	calCreateCmd.Flags().BoolVar(&calForce, "force", false, "Bypass cross-tenant checks")

	// cal delete
	calDeleteCmd.Flags().StringVar(&calAccount, "account", "", "Account")
	calDeleteCmd.Flags().StringVar(&calID, "id", "", "Event ID")

	calCmd.AddCommand(calListCmd)
	calCmd.AddCommand(calCreateCmd)
	calCmd.AddCommand(calDeleteCmd)
}
