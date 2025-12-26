package alerting

func ExplainBurnRate() string {
	return `A burn rate is how fast an SLO consumes its error budget relative to the target window.

Multi-window alerts combine a fast window (catch outages quickly) with a slow window (avoid noise from brief spikes).
The defaults are conservative: they page only when the budget is burning ~14.4x faster over 5m/1h, and create tickets at 6x over 30m/6h.

You should override burn rates only when you have evidence your service tolerates faster budget spend or requires tighter paging, and when your on-call can respond reliably to the added volume.`
}
