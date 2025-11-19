package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/h4ks-com/bean-bank/internal/repository"
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
