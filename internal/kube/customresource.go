package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/cereals-af/cereals-desktop/internal/customresource"
	"github.com/cereals-af/cereals-desktop/internal/resource"
)

// CustomTable delegates additionalPrinterColumns evaluation to the API server.
// Cells remain textual, including int64. Row objects request metadata only.
func (c *Client) CustomTable(ctx context.Context, query resource.Query) (customresource.Page, error) {
	ctx, cancel := c.requestContext(ctx)
	defer cancel()
	path := "/apis/" + query.Group + "/" + query.Version
	if query.Namespace != "" {
		path += "/namespaces/" + query.Namespace
	}
	path += "/" + query.Resource
	stream, err := c.rest.Get().AbsPath(path).Param("limit", "100").Param("continue", query.Continue).Param("labelSelector", query.LabelSelector).Param("includeObject", "Metadata").SetHeader("Accept", "application/json;as=Table;g=meta.k8s.io;v=v1").Stream(ctx)
	if err != nil {
		return customresource.Page{}, requestError(err)
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return customresource.Page{}, errors.New("实例表格读取失败或超过 8 MiB，请缩小筛选范围")
	}
	return decodeCustomTable(data)
}
func decodeCustomTable(data []byte) (customresource.Page, error) {
	var table struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Continue        string `json:"continue"`
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Columns []customresource.Column `json:"columnDefinitions"`
		Rows    []struct {
			Cells  []interface{} `json:"cells"`
			Object struct {
				Metadata struct {
					UID       string `json:"uid"`
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"metadata"`
			} `json:"object"`
		} `json:"rows"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&table); err != nil || table.Kind != "Table" || len(table.Rows) > 100 || len(table.Columns) > 64 {
		return customresource.Page{}, errors.New("服务端表格格式无效或超过边界，请使用通用列表")
	}
	page := customresource.Page{Columns: table.Columns, Rows: []customresource.Row{}, Continue: table.Metadata.Continue, ResourceVersion: table.Metadata.ResourceVersion}
	for _, raw := range table.Rows {
		if raw.Object.Metadata.UID == "" || raw.Object.Metadata.Name == "" {
			return customresource.Page{}, errors.New("服务端表格缺少实例身份，请使用通用列表")
		}
		row := customresource.Row{UID: raw.Object.Metadata.UID, Name: raw.Object.Metadata.Name, Namespace: raw.Object.Metadata.Namespace, Cells: []string{}}
		for _, value := range raw.Cells {
			cell := "—"
			if value != nil {
				if text, ok := value.(string); ok {
					cell = text
				} else {
					encoded, err := json.Marshal(value)
					if err != nil {
						return customresource.Page{}, errors.New("表格单元格无效")
					}
					cell = string(encoded)
				}
			}
			if len(cell) > 2048 {
				cell = cell[:2048] + "…"
			}
			row.Cells = append(row.Cells, cell)
		}
		page.Rows = append(page.Rows, row)
	}
	return page, nil
}
