package httpapi

import "github.com/nikolaykonkin/review-analyzer/internal/storage"

// reviewResponse - DTO для отдачи отзыва через API
// Отделен от storage.Review, чтобы не смешивать слои БД и API
type reviewResponse struct {
	ID          int64    `json:"id"`
	ProductName string   `json:"product_name"`
	BrandName   *string  `json:"brand_name,omitempty"`
	ReviewText  string   `json:"review_text"`
	Rating      int      `json:"rating"`
	Status      string   `json:"status"`
	Sentiment   *string  `json:"sentiment,omitempty"`
	Topics      []string `json:"topics"`
	Priority    *string  `json:"priority,omitempty"`
	Summary     *string  `json:"summary,omitempty"`
}

// newReviewResponse конвертирует доменную модель в DTO
func newReviewResponse(r storage.Review) reviewResponse {
	return reviewResponse{
		ID:          r.ID,
		ProductName: r.ProductName,
		BrandName:   r.BrandName,
		ReviewText:  r.ReviewText,
		Rating:      r.Rating,
		Status:      string(r.Status),
		Sentiment:   r.Sentiment,
		Topics:      r.Topics,
		Priority:    r.Priority,
		Summary:     r.Summary,
	}
}
