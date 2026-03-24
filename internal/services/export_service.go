package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	UserID      uint                   `json:"user_id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	TotalBeans  int                    `json:"total_beans"`
	Transactions []TransactionExportItem `json:"transactions"`
	ExportedAt  time.Time              `json:"exported_at"`
	Signature   string                 `json:"signature"`
}

type TransactionExportItem struct {
	ID          uint      `json:"id"`
	FromUser    string    `json:"from_user"`
	ToUser      string    `json:"to_user"`
	Amount      int       `json:"amount"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
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

	// Header: simple bank icon drawn with rectangles and title
	pdf.SetFillColor(230, 40, 40)
	// roof
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Polygon([]gofpdf.PointType{{X: 20, Y: 20}, {X: 40, Y: 12}, {X: 60, Y: 20}}, "DF")
	// building columns
	pdf.SetFillColor(60, 60, 60)
	pdf.Rect(24, 20, 6, 14, "F")
	pdf.Rect(34, 20, 6, 14, "F")
	pdf.Rect(44, 20, 6, 14, "F")

	// Title
	pdf.SetFont("Arial", "B", 18)
	pdf.SetXY(80, 16)
	pdf.CellFormat(0, 10, "Bean Bank Statement", "", 1, "L", false, 0, "")

	// Subtitle / metadata
	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(80, 24)
	pdf.CellFormat(0, 6, fmt.Sprintf("Generated: %s", export.ExportedAt.Format("2006-01-02 15:04")), "", 1, "L", false, 0, "")
	pdf.SetXY(80, 30)
	pdf.CellFormat(0, 6, "Currency: Beans (fictional)", "", 1, "L", false, 0, "")

	// Account info
	pdf.Ln(6)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 6, "Account Information", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("Username: %s", export.Username), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Email: %s", export.Email), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Current Balance: %d Beans", export.TotalBeans), "", 1, "L", false, 0, "")

	pdf.Ln(4)

	// Transactions table header
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(35, 7, "Date", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 7, "From", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 7, "To", "1", 0, "L", false, 0, "")
	pdf.CellFormat(55, 7, "Note", "1", 0, "L", false, 0, "")
	pdf.CellFormat(20, 7, "Amount", "1", 1, "R", false, 0, "")

	// Transactions
	pdf.SetFont("Arial", "", 10)
	total := 0
	for _, tx := range export.Transactions {
		// date
		dateStr := tx.CreatedAt.Format("2006-01-02")
		pdf.CellFormat(35, 7, dateStr, "1", 0, "L", false, 0, "")
		// from
		pdf.CellFormat(40, 7, tx.FromUser, "1", 0, "L", false, 0, "")
		// to
		pdf.CellFormat(40, 7, tx.ToUser, "1", 0, "L", false, 0, "")
		// note (truncate if long)
		note := tx.Note
		if len(note) > 40 {
			note = note[:37] + "..."
		}
		pdf.CellFormat(55, 7, note, "1", 0, "L", false, 0, "")
		// amount
		amt := strconv.Itoa(tx.Amount) + " Beans"
		pdf.CellFormat(20, 7, amt, "1", 1, "R", false, 0, "")
		total += tx.Amount
		// check for page break
		_, y := pdf.GetXY()
		if y > 270 {
			pdf.AddPage()
		}
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
