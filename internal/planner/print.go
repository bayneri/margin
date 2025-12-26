package planner

import (
	"fmt"
	"io"
)

func Render(w io.Writer, plan Plan) {
	fmt.Fprintf(w, "Project: %s\n", plan.Project)
	fmt.Fprintf(w, "Service: %s\n", plan.Service)
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "SLOs:")
	for _, slo := range plan.SLOs {
		fmt.Fprintf(w, "- %s (objective %.3f%%, window %s, type %s)\n", slo.Name, slo.Objective, slo.Window, slo.SLI.Type)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Alerts:")
	for _, alert := range plan.Alerts {
		fmt.Fprintf(w, "- %s (%s, %v, %.1fx, %s)\n", alert.SLOName, alert.Type, alert.Windows, alert.BurnRate, alert.Severity)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Dashboard: %s\n", plan.Dashboard.ID)
}
