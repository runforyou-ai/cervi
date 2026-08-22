//go:build server

// Package aiprovider 实现模型服务供应商的查询与操作。
package aiprovider

import "github.com/runforyou-ai/cervi/internal/domain"

// Input 定义模型服务供应商可编辑字段。
type Input struct {
	Brand  domain.AIProviderBrand
	Name   string
	APIKey string
	APIURL string
	Models []Model
}

// Model 定义供应商模型目录项。
type Model struct {
	Identifier      string
	Name            string
	Type            domain.AIModelType
	InputModalities []domain.AIModelInputModality
	ContextWindow   int64
	MaxOutputTokens int64
}

// Record 定义模型服务供应商及其模型目录。
type Record struct {
	ID     string
	Brand  domain.AIProviderBrand
	Name   string
	APIKey string
	APIURL string
	Models []Model
}

// Summary 定义模型服务供应商列表项。
type Summary struct {
	ID         string
	Brand      domain.AIProviderBrand
	Name       string
	APIURL     string
	ModelTypes []domain.AIModelType
}
