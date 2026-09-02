package service

import (
	"context"
	"fmt"

	sheets "google.golang.org/api/sheets/v4"
)

// SheetService is the Sheets API surface for spreadsheet metadata. GetSpreadsheet
// returns the spreadsheet with its sheets' properties (title, sheetId, index,
// grid row/column counts) so leaves can enumerate sheets and sizes.
type SheetService interface {
	GetSpreadsheet(ctx context.Context, id string) (*sheets.Spreadsheet, error)
}

// SheetValuesService is the Sheets values surface (values.get/append/update/
// clear). A1 ranges are passed through verbatim; inputOption is passed
// through too — validation of RAW vs USER_ENTERED belongs to the leaf flags,
// not the seam.
type SheetValuesService interface {
	GetValues(ctx context.Context, id, a1Range string) ([][]any, error)
	AppendValues(ctx context.Context, id, a1Range string, values [][]any, inputOption string) (updatedRange string, updatedRows, updatedCols int64, err error)
	UpdateValues(ctx context.Context, id, a1Range string, values [][]any, inputOption string) (updatedRange string, updatedCells int64, err error)
	ClearValues(ctx context.Context, id, a1Range string) (clearedRange string, err error)
}

// GetSpreadsheet returns the spreadsheet's sheets metadata. Fields is
// projected to sheets.properties so each response carries only what the
// leaves render (sheet id, title, index, grid row/column counts).
func (s *realDriveService) GetSpreadsheet(ctx context.Context, id string) (*sheets.Spreadsheet, error) {
	spreadsheet, err := s.sheets.Spreadsheets.Get(id).Fields("sheets.properties").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting spreadsheet %s: %w", id, err)
	}
	return spreadsheet, nil
}

// GetValues reads a range with the API defaults: formatted (human-readable)
// cell values in major-dimension ROWS. An empty (unset) range yields an empty
// (nil) slice — callers distinguish "no data" from "error" by err alone.
func (s *realDriveService) GetValues(ctx context.Context, id, a1Range string) ([][]any, error) {
	resp, err := s.sheets.Spreadsheets.Values.Get(id, a1Range).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting values %s in spreadsheet %s: %w", a1Range, id, err)
	}
	return resp.Values, nil
}

// AppendValues appends rows after the last row of the table containing
// a1Range, labeling the payload with inputOption (RAW|USER_ENTERED; passed
// through verbatim — the leaf owns validation). It returns the API's
// updated range and the row/column counts it reports.
func (s *realDriveService) AppendValues(ctx context.Context, id, a1Range string, values [][]any, inputOption string) (updatedRange string, updatedRows, updatedCols int64, err error) {
	resp, err := s.sheets.Spreadsheets.Values.Append(id, a1Range, &sheets.ValueRange{Values: values}).
		ValueInputOption(inputOption).Context(ctx).Do()
	if err != nil {
		return "", 0, 0, fmt.Errorf("appending values to spreadsheet %s: %w", id, err)
	}
	if resp.Updates == nil {
		return "", 0, 0, nil
	}
	return resp.Updates.UpdatedRange, resp.Updates.UpdatedRows, resp.Updates.UpdatedColumns, nil
}

// UpdateValues writes rows starting at the top-left of a1Range, with the
// same inputOption passthrough as AppendValues. It returns the updated range
// and the number of cells written.
func (s *realDriveService) UpdateValues(ctx context.Context, id, a1Range string, values [][]any, inputOption string) (updatedRange string, updatedCells int64, err error) {
	resp, err := s.sheets.Spreadsheets.Values.Update(id, a1Range, &sheets.ValueRange{Values: values}).
		ValueInputOption(inputOption).Context(ctx).Do()
	if err != nil {
		return "", 0, fmt.Errorf("updating values in spreadsheet %s: %w", id, err)
	}
	return resp.UpdatedRange, resp.UpdatedCells, nil
}

// ClearValues empties every cell in a1Range (formatting is kept) and returns
// the range that was actually cleared (bounded to the sheet for unbounded
// ranges).
func (s *realDriveService) ClearValues(ctx context.Context, id, a1Range string) (clearedRange string, err error) {
	resp, err := s.sheets.Spreadsheets.Values.Clear(id, a1Range, &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("clearing values %s in spreadsheet %s: %w", a1Range, id, err)
	}
	return resp.ClearedRange, nil
}
