package service

import (
	"context"
	"fmt"

	sheets "google.golang.org/api/sheets/v4"
)

// SheetTabService is the Sheets worksheet-tab management surface (the tabs
// are the worksheets): add one by title, delete or rename one by its exact
// current title. Listing is SheetService.GetSpreadsheet; values are
// SheetValuesService — this interface is management only.
type SheetTabService interface {
	AddSheetTab(ctx context.Context, spreadsheetID, title string) (sheetID int64, err error)
	DeleteSheetTab(ctx context.Context, spreadsheetID, title string) error
	RenameSheetTab(ctx context.Context, spreadsheetID, oldTitle, newTitle string) error
}

// AddSheetTab appends a new worksheet tab with the given title to the
// spreadsheet in ONE batchUpdate carrying a single AddSheetRequest, and
// returns the new sheet's id from the addSheet reply.
func (s *realDriveService) AddSheetTab(ctx context.Context, spreadsheetID, title string) (int64, error) {
	resp, err := s.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{Title: title},
			},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("adding sheet %q to spreadsheet %s: %w", title, spreadsheetID, err)
	}
	if len(resp.Replies) == 0 || resp.Replies[0].AddSheet == nil || resp.Replies[0].AddSheet.Properties == nil {
		return 0, fmt.Errorf("adding sheet %q to spreadsheet %s: no new sheet id in the reply", title, spreadsheetID)
	}
	return resp.Replies[0].AddSheet.Properties.SheetId, nil
}

// DeleteSheetTab deletes the worksheet tab titled title: it resolves the
// exact (case-sensitive) title to its numeric sheetId from the spreadsheet
// metadata first, then issues ONE batchUpdate carrying a single
// DeleteSheetRequest. An unknown title fails before any write is sent.
func (s *realDriveService) DeleteSheetTab(ctx context.Context, spreadsheetID, title string) error {
	sheetID, err := s.sheetIDForTitle(ctx, spreadsheetID, title)
	if err != nil {
		return err
	}
	if _, err := s.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			DeleteSheet: &sheets.DeleteSheetRequest{
				// The API's deleteSheet takes the numeric id; the generated
				// struct omits a zero value from the wire, which would drop
				// the first sheet's valid id 0.
				SheetId:         sheetID,
				ForceSendFields: []string{"SheetId"},
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("deleting sheet %q from spreadsheet %s: %w", title, spreadsheetID, err)
	}
	return nil
}

// RenameSheetTab renames the worksheet tab titled oldTitle to newTitle: it
// resolves the exact (case-sensitive) old title to its numeric sheetId from
// the spreadsheet metadata first, then issues ONE batchUpdate carrying a
// single UpdateSheetPropertiesRequest with Fields "title". An unknown old
// title fails before any write is sent.
func (s *realDriveService) RenameSheetTab(ctx context.Context, spreadsheetID, oldTitle, newTitle string) error {
	sheetID, err := s.sheetIDForTitle(ctx, spreadsheetID, oldTitle)
	if err != nil {
		return err
	}
	if _, err := s.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: sheetID,
					Title:   newTitle,
					// Same omitempty hazard as deleteSheet: the sheet to
					// update is identified by properties.sheetId, so a valid
					// 0 must reach the wire.
					ForceSendFields: []string{"SheetId"},
				},
				Fields: "title",
			},
		}},
	}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("renaming sheet %q to %q in spreadsheet %s: %w", oldTitle, newTitle, spreadsheetID, err)
	}
	return nil
}

// sheetIDForTitle returns the numeric sheetId of the worksheet whose title
// matches exactly (case-sensitively). Matching happens on the metadata
// GetSpreadsheet already serves, so an unknown title errors — naming the
// title — before any write is issued.
func (s *realDriveService) sheetIDForTitle(ctx context.Context, spreadsheetID, title string) (int64, error) {
	spreadsheet, err := s.GetSpreadsheet(ctx, spreadsheetID)
	if err != nil {
		return 0, err
	}
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties != nil && sheet.Properties.Title == title {
			return sheet.Properties.SheetId, nil
		}
	}
	return 0, fmt.Errorf("spreadsheet %s has no sheet named %q", spreadsheetID, title)
}
