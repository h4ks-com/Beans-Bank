package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/h4ks-com/bean-bank/internal/repository"
	"github.com/jung-kurt/gofpdf"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidExport    = errors.New("invalid export data")
)

type TransactionExport struct {
	UserID       uint                    `json:"user_id"`
	Username     string                  `json:"username"`
	Email        string                  `json:"email"`
	TotalBeans   int                     `json:"total_beans"`
	Transactions []TransactionExportItem `json:"transactions"`
	ExportedAt   time.Time               `json:"exported_at"`
	Signature    string                  `json:"signature"`
}

type TransactionExportItem struct {
	ID        uint      `json:"id"`
	FromUser  string    `json:"from_user"`
	ToUser    string    `json:"to_user"`
	Amount    int       `json:"amount"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ExportService struct {
	userRepo        *repository.UserRepository
	transactionRepo *repository.TransactionRepository
	signingKey      string
}

func NewExportService(userRepo *repository.UserRepository, transactionRepo *repository.TransactionRepository, signingKey string) *ExportService {
	return &ExportService{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
		signingKey:      signingKey,
	}
}

func (s *ExportService) ExportTransactions(username string) (*TransactionExport, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	transactions, err := s.transactionRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	exportItems := make([]TransactionExportItem, len(transactions))
	for i, tx := range transactions {
		exportItems[i] = TransactionExportItem{
			ID:        tx.ID,
			FromUser:  tx.FromUser.Username,
			ToUser:    tx.ToUser.Username,
			Amount:    tx.Amount,
			Note:      tx.Note,
			CreatedAt: tx.CreatedAt,
		}
	}

	export := &TransactionExport{
		UserID:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		TotalBeans:   user.BeanAmount,
		Transactions: exportItems,
		ExportedAt:   time.Now(),
	}

	signature, err := s.signExport(export)
	if err != nil {
		return nil, err
	}
	export.Signature = signature

	return export, nil
}

func (s *ExportService) VerifyExport(exportData []byte, signature string) (bool, error) {
	var export TransactionExport
	err := json.Unmarshal(exportData, &export)
	if err != nil {
		return false, ErrInvalidExport
	}

	export.Signature = ""

	computedSignature, err := s.signExport(&export)
	if err != nil {
		return false, err
	}

	return hmac.Equal([]byte(computedSignature), []byte(signature)), nil
}

func (s *ExportService) VerifyExportData(exportData *TransactionExport) (bool, error) {
	if exportData.Signature == "" {
		return false, ErrInvalidExport
	}

	providedSignature := exportData.Signature

	exportCopy := *exportData
	exportCopy.Signature = ""

	computedSignature, err := s.signExport(&exportCopy)
	if err != nil {
		return false, err
	}

	return hmac.Equal([]byte(computedSignature), []byte(providedSignature)), nil
}

func (s *ExportService) signExport(export *TransactionExport) (string, error) {
	exportCopy := *export
	exportCopy.Signature = ""

	data, err := json.Marshal(exportCopy)
	if err != nil {
		return "", err
	}

	h := hmac.New(sha256.New, []byte(s.signingKey))
	h.Write(data)
	signature := hex.EncodeToString(h.Sum(nil))

	return signature, nil
}

func reverseString(s string) []rune {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

// GenerateStatementPDF builds a simple, realistic-looking PDF bank statement
// for the given username. The currency is explicitly stated as "Beans".
func (s *ExportService) GenerateStatementPDF(username string) ([]byte, error) {
	export, err := s.ExportTransactions(username)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	pageW, pageH := pdf.GetPageSize()
	left, _, right, bottom := pdf.GetMargins()
	contentW := pageW - left - right

	// Header: try to use embedded Font Awesome solid font (fa-solid-900.ttf) for the coins glyph;
	// fall back to the drawn coins if embedding fails.
	pdf.SetTextColor(25, 25, 25)
	headerY := 18.0

	// attempt to register Font Awesome TTF bundled in web/static/fonts
	faPath := "web/static/fonts/fa-solid-900.ttf"
	if _, err := os.Stat(faPath); err == nil {
		// AddUTF8Font registers the font family for the current PDF.
		// This function does not return an error value in this gofpdf version.
		pdf.AddUTF8Font("fa", "", faPath)
		// Set font and draw glyph (FA coins U+F51E) at left
		pdf.SetFont("fa", "", 20)
		pdf.SetXY(left+6, headerY-8)
		pdf.CellFormat(0, 10, string('\uf51e'), "", 0, "L", false, 0, "")
	} else {
		// draw coins (three circles with slight offsets)
		pdf.SetFont("Arial", "B", 20)
		coinCx := left + 12.0
		for i := 0; i < 3; i++ {
			cx := coinCx + float64(i)*6.0
			cy := headerY
			r := 6.0
			// coin body
			pdf.SetFillColor(int(212-i*10), int(142-i*8), int(42-i*6))
			pdf.SetDrawColor(80, 60, 30)
			pdf.SetLineWidth(0.5)
			pdf.Circle(cx, cy, r, "F")
			// rim
			pdf.SetDrawColor(120, 95, 45)
			pdf.Circle(cx, cy, r, "D")
			// highlight
			pdf.SetFillColor(255, 255, 255)
			pdf.Ellipse(cx-2.0, cy-1.5, 2.5, 1.5, 0.0, "F")
		}
	}

	pdf.SetXY(left+36, headerY-6)
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(0, 10, "Bean Bank Statement", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(left+36, headerY+4)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated: %s", export.ExportedAt.Format("2006-01-02 15:04")), "", 1, "L", false, 0, "")
	pdf.SetXY(left+36, headerY+9)
	pdf.CellFormat(0, 5, "Currency: Beans (fictional)", "", 1, "L", false, 0, "")

	// Account info block
	pdf.Ln(6)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 6, "Account Information", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("Username: %s", export.Username), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Email: %s", export.Email), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Current Balance: %d Beans", export.TotalBeans), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Transactions: draw inside a rounded container with gentle separators
	formatNumber := func(n int) string {
		s := strconv.Itoa(n)
		if n < 0 {
			s = s[1:]
		}
		out := ""
		for i, c := range reverseString(s) {
			if i > 0 && i%3 == 0 {
				out = "," + out
			}
			out = string(c) + out
		}
		if n < 0 {
			out = "-" + out
		}
		return out
	}

	pdf.SetFont("Arial", "", 10)
	rowH := 10.0

	tableX := left
	tableY := pdf.GetY()
	// columns: Date, From, To, Note, Amount
	cwDate := 30.0
	cwFrom := 40.0
	cwTo := 40.0
	cwAmt := 45.0
	cwNote := contentW - cwDate - cwFrom - cwTo - cwAmt

	headerH := rowH

	// Pagination: split rows across pages with repeated headers
	total := 0
	rowsPerPage := int((pageH - bottom - tableY - 40) / rowH) // leave room for footer/notes
	if rowsPerPage < 3 {
		rowsPerPage = 3
	}

	numRows := len(export.Transactions)
	pages := (numRows + rowsPerPage - 1) / rowsPerPage
	rowIndex := 0

	for p := 0; p < pages; p++ {
		if p > 0 {
			pdf.AddPage()
			tableY = pdf.GetY()
		}
		// compute how many rows on this page
		remaining := numRows - rowIndex
		thisRows := rowsPerPage
		if remaining < thisRows {
			thisRows = remaining
		}

		// container height for this page
		containerHPage := headerH + float64(thisRows)*rowH + 12.0
		if containerHPage > pageH-bottom-tableY {
			containerHPage = pageH - bottom - tableY - 10
		}

		// draw container
		pdf.SetFillColor(255, 255, 255)
		pdf.SetDrawColor(190, 190, 190)
		pdf.SetLineWidth(0.7)
		pdf.RoundedRect(tableX-1, tableY-1, contentW+2, containerHPage+2, 5.0, "1111", "DF")

		// header band
		pdf.SetFillColor(245, 245, 245)
		pdf.Rect(tableX, tableY, contentW, headerH, "F")

		// headers text
		pdf.SetFont("Arial", "B", 11)
		x := tableX
		pdf.SetXY(x+4, tableY+3)
		pdf.CellFormat(cwDate-8, headerH-6, "Date", "", 0, "L", false, 0, "")
		x += cwDate
		pdf.SetXY(x+4, tableY+3)
		pdf.CellFormat(cwFrom-8, headerH-6, "From", "", 0, "L", false, 0, "")
		x += cwFrom
		pdf.SetXY(x+4, tableY+3)
		pdf.CellFormat(cwTo-8, headerH-6, "To", "", 0, "L", false, 0, "")
		x += cwTo
		pdf.SetXY(x+4, tableY+3)
		pdf.CellFormat(cwNote-8, headerH-6, "Note", "", 0, "L", false, 0, "")
		x += cwNote
		pdf.SetXY(x+4, tableY+3)
		pdf.CellFormat(cwAmt-8, headerH-6, "Amount", "", 0, "R", false, 0, "")

		// vertical separators (soft)
		sepX := tableX
		pdf.SetDrawColor(230, 230, 230)
		for _, w := range []float64{cwDate, cwFrom, cwTo, cwNote, cwAmt} {
			sepX += w
			pdf.Line(sepX, tableY, sepX, tableY+containerHPage)
		}

		// rows for this page
		pdf.SetFont("Arial", "", 10)
		for i := 0; i < thisRows; i++ {
			tx := export.Transactions[rowIndex]
			ry := tableY + headerH + float64(i)*rowH
			x = tableX
			pdf.SetXY(x+4, ry+3)
			pdf.CellFormat(cwDate-8, rowH-6, tx.CreatedAt.Format("2006-01-02"), "", 0, "L", false, 0, "")
			x += cwDate
			pdf.SetXY(x+4, ry+3)
			pdf.CellFormat(cwFrom-8, rowH-6, tx.FromUser, "", 0, "L", false, 0, "")
			x += cwFrom
			pdf.SetXY(x+4, ry+3)
			pdf.CellFormat(cwTo-8, rowH-6, tx.ToUser, "", 0, "L", false, 0, "")
			x += cwTo
			note := tx.Note
			if len(note) > 120 {
				note = note[:117] + "..."
			}
			pdf.SetXY(x+4, ry+3)
			pdf.CellFormat(cwNote-8, rowH-6, note, "", 0, "L", false, 0, "")
			x += cwNote
			pdf.SetXY(x+4, ry+3)
			pdf.CellFormat(cwAmt-10, rowH-6, formatNumber(tx.Amount)+" Beans", "", 0, "R", false, 0, "")
			// row separator
			pdf.SetDrawColor(240, 240, 240)
			pdf.Line(tableX, ry+rowH, tableX+contentW, ry+rowH)
			total += tx.Amount
			rowIndex++
		}

		// move cursor after container for this page
		pdf.SetXY(tableX, tableY+containerHPage+6)
	}

	// Totals
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(170, 8, "Total (listed)", "1", 0, "R", false, 0, "")
	pdf.CellFormat(20, 8, fmt.Sprintf("%d Beans", total), "1", 1, "R", false, 0, "")

	pdf.Ln(6)
	pdf.SetFont("Arial", "I", 9)
	pdf.MultiCell(0, 5, "This statement is for demonstration purposes. The currency used by Bean Bank is 'Beans' and is fictional. This document is not a real bank statement.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
