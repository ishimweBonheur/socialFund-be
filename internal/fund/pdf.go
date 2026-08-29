package fund

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// StatementPDF creates a branded, printable, multi-page transaction statement.
func StatementPDF(title, reference, dateFrom, dateTo string, transactions []FundTransaction) []byte {
	const rowsPerPage = 24
	pages := max(1, (len(transactions)+rowsPerPage-1)/rowsPerPage)
	objects := map[int]string{
		1: "<< /Type /Catalog /Pages 2 0 R >>",
		3: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		4: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
	}
	pageRefs := make([]string, 0, pages)
	for page := 0; page < pages; page++ {
		pageID, contentID := 5+page*2, 6+page*2
		pageRefs = append(pageRefs, fmt.Sprintf("%d 0 R", pageID))
		start, end := page*rowsPerPage, min((page+1)*rowsPerPage, len(transactions))
		stream := statementPage(title, reference, dateFrom, dateTo, page+1, pages, transactions, transactions[start:end])
		objects[pageID] = fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 842 595] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", contentID)
		objects[contentID] = fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream)
	}
	objects[2] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(pageRefs, " "), pages)
	return assemblePDF(objects)
}

func statementPage(title, reference, dateFrom, dateTo string, page, pages int, all, rows []FundTransaction) string {
	var b strings.Builder
	// Navy header and brand title.
	b.WriteString("0.055 0.145 0.255 rg 0 505 842 90 re f\n")
	textAt(&b, 32, 557, 20, true, "SOCIAL FUND")
	textAt(&b, 32, 532, 11, false, title)
	textAt(&b, 630, 557, 9, true, reference)
	textAt(&b, 630, 540, 8, false, "Generated "+time.Now().UTC().Format("2006-01-02 15:04 UTC"))
	period := "All dates"
	if dateFrom != "" || dateTo != "" {
		period = valueOr(dateFrom, "Beginning") + " to " + valueOr(dateTo, "Today")
	}
	textAt(&b, 630, 524, 8, false, "Period: "+period)

	var totalIn, totalOut decimal.Decimal
	for _, item := range all {
		if item.Direction == "IN" {
			totalIn = totalIn.Add(item.Amount)
		} else {
			totalOut = totalOut.Add(item.Amount)
		}
	}
	cards := []struct {
		x            float64
		label, value string
	}{{32, "TOTAL IN", "RWF " + totalIn.StringFixed(2)}, {220, "TOTAL OUT", "RWF " + totalOut.StringFixed(2)}, {408, "NET BALANCE", "RWF " + totalIn.Sub(totalOut).StringFixed(2)}, {596, "TRANSACTIONS", fmt.Sprint(len(all))}}
	for _, card := range cards {
		b.WriteString(fmt.Sprintf("0.95 0.97 0.98 rg %.0f 450 170 42 re f\n", card.x))
		textAt(&b, card.x+12, 476, 7, true, card.label)
		textAt(&b, card.x+12, 459, 12, true, card.value)
	}

	// Table heading.
	b.WriteString("0.12 0.38 0.55 rg 32 414 778 24 re f\n")
	headings := []struct {
		x    float64
		text string
	}{{40, "DATE"}, {130, "MEMBER"}, {285, "TYPE"}, {370, "FLOW"}, {415, "AMOUNT"}, {505, "METHOD"}, {610, "REFERENCE"}, {750, "STATUS"}}
	for _, heading := range headings {
		textAtColor(&b, heading.x, 423, 7, true, heading.text, "1 1 1")
	}
	if len(rows) == 0 {
		textAt(&b, 40, 385, 10, false, "No completed transactions found for this period.")
	}
	for index, item := range rows {
		y := 394 - float64(index*14)
		if index%2 == 0 {
			b.WriteString(fmt.Sprintf("0.965 0.975 0.985 rg 32 %.0f 778 14 re f\n", y-4))
		}
		values := []struct {
			x     float64
			width int
			text  string
		}{{40, 15, item.CreatedAt.UTC().Format("2006-01-02")}, {130, 24, item.UserName}, {285, 13, item.Type}, {370, 5, item.Direction}, {415, 13, "RWF " + item.Amount.StringFixed(2)}, {505, 15, pointerValue(item.PaymentMethod)}, {610, 21, pointerValue(item.Reference)}, {750, 10, item.Status}}
		for _, value := range values {
			textAt(&b, value.x, y, 7, false, truncate(value.text, value.width))
		}
	}
	b.WriteString("0.82 0.86 0.89 RG 32 42 m 810 42 l S\n")
	textAt(&b, 32, 25, 7, false, "Keep this statement as transaction evidence. Verify references with the Social Fund administrator.")
	textAt(&b, 760, 25, 7, true, fmt.Sprintf("Page %d of %d", page, pages))
	return b.String()
}

func textAt(b *strings.Builder, x, y float64, size int, bold bool, value string) {
	textAtColor(b, x, y, size, bold, value, "0.10 0.14 0.18")
}
func textAtColor(b *strings.Builder, x, y float64, size int, bold bool, value, color string) {
	font := "F1"
	if bold {
		font = "F2"
	}
	fmt.Fprintf(b, "BT /%s %d Tf %s rg 1 0 0 1 %.1f %.1f Tm (%s) Tj ET\n", font, size, color, x, y, pdfEscape(value))
}
func truncate(value string, width int) string {
	value = strings.TrimSpace(value)
	if len(value) <= width {
		return value
	}
	if width < 4 {
		return value[:width]
	}
	return value[:width-3] + "..."
}
func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func pointerValue(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}
	return *value
}
func pdfEscape(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, value)
	return strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(value)
}
func assemblePDF(objects map[int]string) []byte {
	ids := make([]int, 0, len(objects))
	for id := range objects {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, ids[len(ids)-1]+1)
	for _, id := range ids {
		offsets[id] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", id, objects[id])
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for id := 1; id < len(offsets); id++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return out.Bytes()
}
