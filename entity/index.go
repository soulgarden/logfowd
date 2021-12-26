package entity

import "time"

type IndexRequest struct {
	IndexRequestBody *IndexRequestBody `json:"index"`
}

type DynamicTemplates struct {
	Timestamp string `json:"@timestamp"`
}

type IndexRequestBody struct {
	ID string `json:"_id"`
}

type FieldsBody struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"@timestamp"`
}
