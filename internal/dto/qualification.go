package dto

import (
	"github.com/google/uuid"
)

type CreateQualificationRequest struct {
	Name            string `json:"name"`
	IssuingBody     string `json:"issuing_body,omitempty"`
	RequiredForRole string `json:"required_for_role,omitempty"`
}

type UpdateQualificationRequest struct {
	Name            string `json:"name,omitempty"`
	IssuingBody     string `json:"issuing_body,omitempty"`
	RequiredForRole string `json:"required_for_role,omitempty"`
}

type QualificationResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	IssuingBody     string    `json:"issuing_body,omitempty"`
	RequiredForRole string    `json:"required_for_role,omitempty"`
	CreatedAt       string    `json:"created_at"`
}

type AddEmployeeQualificationRequest struct {
	QualificationID uuid.UUID `json:"qualification_id"`
	IssueDate       *string   `json:"issue_date,omitempty"`   // "YYYY-MM-DD"
	ExpiryDate      *string   `json:"expiry_date,omitempty"`  // "YYYY-MM-DD"
	DocumentURL     string    `json:"document_url,omitempty"`
}

type UpdateEmployeeQualificationRequest struct {
	IssueDate   *string `json:"issue_date,omitempty"`   // "YYYY-MM-DD"
	ExpiryDate  *string `json:"expiry_date,omitempty"`  // "YYYY-MM-DD"
	Verified    *bool   `json:"verified,omitempty"`
	DocumentURL string  `json:"document_url,omitempty"`
}

type EmployeeQualificationResponse struct {
	ID                uuid.UUID `json:"id"`
	EmployeeID        uuid.UUID `json:"employee_id"`
	QualificationID   uuid.UUID `json:"qualification_id"`
	QualificationName string    `json:"qualification_name,omitempty"`
	IssueDate         *string   `json:"issue_date,omitempty"`
	ExpiryDate        *string   `json:"expiry_date,omitempty"`
	Verified          bool      `json:"verified"`
	ExpiringInDays    *int      `json:"expiring_in_days,omitempty"` // set if expiring within 30 days
	DocumentURL       string    `json:"document_url,omitempty"`
	CreatedAt         string    `json:"created_at"`
}

type QualificationListResponse struct {
	Qualifications []QualificationResponse `json:"qualifications"`
}

type EmployeeQualificationListResponse struct {
	Qualifications []EmployeeQualificationResponse `json:"qualifications"`
}
