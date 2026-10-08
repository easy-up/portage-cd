package pipelines

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"strings"
	"unicode"
)

// Values of verdict.decision in a webhook response
const (
	VerdictDecisionPass    = "pass"
	VerdictDecisionFail    = "fail"
	VerdictDecisionPending = "pending"
)

// Values of verdict.reasons[].status in a webhook response
const (
	VerdictReasonPass     = "pass"
	VerdictReasonFail     = "fail"
	VerdictReasonAccepted = "accepted"
)

const (
	maxVerdictTextLength = 300
	maxVerdictReasons    = 50
)

// webhookResponse is the optional JSON body a deploy webhook can return
//
//	{"verdict": {"decision": "fail", "summary": "...", "reasons": [{"rule": "grype.critical", "status": "fail", "message": "3 critical (limit 0)"}], "detailsUrl": "https://..."}}
type webhookResponse struct {
	Verdict *Verdict `json:"verdict"`
}

// Verdict is the deployment decision returned by a webhook receiver
type Verdict struct {
	Decision   string          `json:"decision"`
	Summary    string          `json:"summary"`
	Reasons    []VerdictReason `json:"reasons"`
	DetailsURL string          `json:"detailsUrl"`
}

// VerdictReason is one rule evaluated by the webhook receiver
type VerdictReason struct {
	Rule    string `json:"rule"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// parseVerdict returns the verdict in a webhook response body, or nil when there is none
//
// A missing, non-JSON, or unrecognised body is not an error: receivers are not required to return a verdict.
func parseVerdict(contentType string, body []byte) *Verdict {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return nil
	}
	var res webhookResponse
	if err := json.Unmarshal(body, &res); err != nil || res.Verdict == nil {
		return nil
	}
	decision := strings.ToLower(strings.TrimSpace(res.Verdict.Decision))
	switch decision {
	case VerdictDecisionPass, VerdictDecisionFail, VerdictDecisionPending:
		res.Verdict.Decision = decision
	default:
		return nil
	}
	return res.Verdict
}

// writeVerdict prints a verdict for the CI log
//
// Only the known fields are printed, each reduced to a single line of printable text,
// so a response cannot inject CI commands or dump arbitrary content into the log.
func writeVerdict(w io.Writer, v *Verdict) {
	header := "Deploy verdict: " + strings.ToUpper(v.Decision)
	if summary := sanitizeVerdictText(v.Summary); summary != "" {
		header += " - " + summary
	}
	fmt.Fprintln(w, header)

	reasons := v.Reasons
	if len(reasons) > maxVerdictReasons {
		reasons = reasons[:maxVerdictReasons]
	}
	width := 0
	for _, r := range reasons {
		width = max(width, len(sanitizeVerdictText(r.Rule)))
	}
	for _, r := range reasons {
		rule := sanitizeVerdictText(r.Rule)
		message := sanitizeVerdictText(r.Message)
		fmt.Fprintf(w, "  %s %-*s  %s\n", reasonMarker(r.Status), width, rule, message)
	}
	if len(v.Reasons) > maxVerdictReasons {
		fmt.Fprintf(w, "  ... %d more\n", len(v.Reasons)-maxVerdictReasons)
	}
	if v.DetailsURL != "" {
		fmt.Fprintln(w, "  Details: "+redactURL(sanitizeVerdictText(v.DetailsURL)))
	}
}

func reasonMarker(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case VerdictReasonPass:
		return "[pass]    "
	case VerdictReasonFail:
		return "[fail]    "
	case VerdictReasonAccepted:
		return "[accepted]"
	}
	return "[-]       "
}

// sanitizeVerdictText collapses text to one printable line of bounded length
func sanitizeVerdictText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > maxVerdictTextLength {
		s = string(runes[:maxVerdictTextLength]) + "..."
	}
	return s
}
