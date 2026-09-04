//go:build server

package knowledgeretrieval

import (
	"context"
	"errors"
	"testing"
)

// TestSearchFusesMultipleSourcesAndQueries 验证多库多查询会去重并按 RRF 稳定融合。
func TestSearchFusesMultipleSourcesAndQueries(t *testing.T) {
	sources := []Source{
		{ID: "base-a", Name: "手册", Retrieve: func(_ context.Context, query string) ([]Record, error) {
			if query == "休假" {
				return []Record{{SegmentID: "shared", Content: "共同结果"}, {SegmentID: "leave", Content: "休假结果"}}, nil
			}
			return []Record{{SegmentID: "shared", Content: "共同结果"}}, nil
		}},
		{ID: "base-b", Name: "制度", Retrieve: func(context.Context, string) ([]Record, error) {
			return []Record{{SegmentID: "expense", Content: "报销结果"}}, nil
		}},
	}
	result, err := Search(context.Background(), sources, Request{Queries: []string{"休假", "报销", "休假"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 3 || result.Records[0].SegmentID != "shared" {
		t.Fatalf("records = %#v", result.Records)
	}
	if indexes := result.Records[0].MatchedQueries; len(indexes) != 3 || indexes[0] != 1 || indexes[1] != 2 || indexes[2] != 3 {
		t.Fatalf("matched query indexes = %#v", indexes)
	}
}

// TestSearchKeepsSuccessfulSources 验证部分来源失败时仍返回其他来源的结果。
func TestSearchKeepsSuccessfulSources(t *testing.T) {
	sources := []Source{
		{ID: "failed", Retrieve: func(context.Context, string) ([]Record, error) { return nil, errors.New("failed") }},
		{ID: "available", Retrieve: func(context.Context, string) ([]Record, error) { return []Record{{SegmentID: "result"}}, nil }},
	}
	result, err := Search(context.Background(), sources, Request{Queries: []string{"查询"}})
	if err != nil || len(result.Records) != 1 || result.Records[0].KnowledgeBaseID != "available" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

// TestSearchReadsCursorContext 验证游标模式直接读取指定知识库中的周边分段。
func TestSearchReadsCursorContext(t *testing.T) {
	sources := []Source{{ID: "base-a", Read: func(_ context.Context, cursor Cursor, before, after int) ([]Record, error) {
		if cursor.SegmentID != "segment-2" || before != 1 || after != 2 {
			t.Fatalf("cursor = %#v, before = %d, after = %d", cursor, before, after)
		}
		return []Record{{SegmentID: "segment-1"}, {SegmentID: "segment-2", Matched: true}}, nil
	}}}
	result, err := Search(context.Background(), sources, Request{
		Cursor: &Cursor{KnowledgeBaseID: "base-a", DocumentID: "document-1", SegmentID: "segment-2", Position: 2},
		Before: 1, After: 2,
	})
	if err != nil || len(result.Records) != 2 || !result.Records[1].Matched {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

// TestSearchRejectsQueriesWithCursor 验证关键词和游标不能在同一次查询中混用。
func TestSearchRejectsQueriesWithCursor(t *testing.T) {
	cursor := &Cursor{KnowledgeBaseID: " base-a ", DocumentID: " document-1 ", SegmentID: " segment-2 ", Position: 2}
	_, err := Search(context.Background(), nil, Request{Queries: []string{"查询"}, Cursor: cursor})
	if err == nil {
		t.Fatal("queries and cursor should be rejected")
	}
	if cursor.KnowledgeBaseID != " base-a " || cursor.DocumentID != " document-1 " || cursor.SegmentID != " segment-2 " {
		t.Fatalf("cursor was modified: %#v", cursor)
	}
}
