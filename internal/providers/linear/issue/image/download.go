package image

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/providers/linear/service"
)

// fallbackFilename names an image whose alt text sanitized to nothing —
// storage URL paths are bare UUIDs, so there is nothing else to take the
// name from.
const fallbackFilename = "image"

// newDownloadCmd returns `linear issue image download`: discover the
// issue's images exactly like `image list` and download each through the
// FileService seam into --out-dir, one row per image.
func newDownloadCmd(cfg *app.Config, d Dialers) *cobra.Command {
	var withComments bool
	var outDir string
	cmd := &cobra.Command{
		Use:   "download <id>",
		Short: "Download images referenced by a Linear issue",
		Example: `# Download the issue's description images into the current directory
everything-cli linear issue image download BLA-123

# Include comment images, writing into a directory
everything-cli linear issue image download BLA-123 --comments --out-dir shots`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := discoverRefs(cmd.Context(), d, args[0], withComments)
			if err != nil {
				return err
			}
			fileSvc, err := d.File(cmd.Context())
			if err != nil {
				return err
			}
			rows, failed := downloadImages(cmd.Context(), cfg.Fs, fileSvc, refs, outDir)
			printImageDownloads(cmd, cfg, rows)
			if failed > 0 {
				return fmt.Errorf("image download: %d of %d downloads failed", failed, len(refs))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&withComments, "comments", false, "Also scan comment bodies")
	cmd.Flags().StringVar(&outDir, "out-dir", ".", "Directory to write images into (created as needed)")
	return cmd
}

// downloadImages downloads every discovered ref into outDir on fs,
// returning one row per ref in discovery order plus the count that
// failed. A failed ref — a non-allowlisted URL or a failed fetch, both
// named by the FileService seam — fails only its own row and leaves no
// partial file; the rest of the run proceeds.
func downloadImages(ctx context.Context, fs afero.Fs, fileSvc service.FileService, refs []imageRef, outDir string) ([]downloadRow, int) {
	dl := &imageDownloader{fs: fs, fileSvc: fileSvc, outDir: outDir, taken: map[string]bool{}}
	rows := make([]downloadRow, 0, len(refs))
	failed := 0
	for _, ref := range refs {
		row := dl.download(ctx, ref)
		if row.Err != "" {
			failed++
		}
		rows = append(rows, row)
	}
	return rows, failed
}

// downloadRow is one rendered download outcome: the fields of a successful
// row, or the named error of a failed one.
type downloadRow struct {
	Filename string
	Path     string
	Bytes    int64
	Err      string
}

// imageDownloader carries one run's download state: the destination
// directory, the file seam, and the names already taken — the last keeps
// the run collision-safe.
type imageDownloader struct {
	fs      afero.Fs
	fileSvc service.FileService
	outDir  string
	taken   map[string]bool
}

// download downloads one ref and returns its row. The final name is only
// fully known once the response arrived (a missing extension is fixed
// from its Content-Type), so the bytes stream under a claimed scratch
// name and the file is renamed when the final name differs. On failure
// the scratch file is removed — a failed row leaves no partial file.
func (dl *imageDownloader) download(ctx context.Context, ref imageRef) downloadRow {
	base := sanitizeFilename(ref.Alt)
	if base == "" {
		base = fallbackFilename
	}
	scratch := dl.claim(base)
	scratchPath := path.Join(dl.outDir, scratch)
	var meta service.FileMeta
	err := app.WriteToFile(dl.fs, scratchPath, func(w io.Writer) error {
		var err error
		meta, err = dl.fileSvc.DownloadFileMeta(ctx, ref.URL, w)
		return err
	})
	if err != nil {
		_ = dl.fs.Remove(scratchPath)
		return downloadRow{Filename: scratch, Err: err.Error()}
	}
	name := dl.finalName(scratch, meta.ContentType)
	if name != scratch {
		if err := dl.fs.Rename(scratchPath, path.Join(dl.outDir, name)); err != nil {
			// The bytes are on disk; surface the rename failure rather than
			// pretend nothing was written.
			return downloadRow{Filename: scratch, Path: scratchPath, Bytes: meta.Bytes,
				Err: fmt.Sprintf("renaming to %s: %v", name, err)}
		}
	}
	return downloadRow{Filename: name, Path: path.Join(dl.outDir, name), Bytes: meta.Bytes}
}

// finalName fixes a missing extension from the response's Content-Type
// (the alt text usually carries the original filename; when it does not,
// the response's type names the file) and claims the result so two refs
// cannot write the same file within one run. A name that already has an
// extension, or a response type with no known image extension, keeps the
// scratch name.
func (dl *imageDownloader) finalName(scratch, contentType string) string {
	if path.Ext(scratch) != "" {
		return scratch
	}
	ext := extensionFor(contentType)
	if ext == "" {
		return scratch
	}
	return dl.claim(scratch + "." + ext)
}

// claim marks name taken and returns it, or the first free variant with a
// -2, -3, ... suffix before the extension.
func (dl *imageDownloader) claim(name string) string {
	if !dl.taken[name] {
		dl.taken[name] = true
		return name
	}
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if !dl.taken[candidate] {
			dl.taken[candidate] = true
			return candidate
		}
	}
}

// contentTypeExts maps the Content-Types Linear's file storage serves for
// images to filename extensions, used only to fix a missing extension.
var contentTypeExts = map[string]string{
	"image/avif":    "avif",
	"image/bmp":     "bmp",
	"image/gif":     "gif",
	"image/jpeg":    "jpg",
	"image/png":     "png",
	"image/svg+xml": "svg",
	"image/tiff":    "tiff",
	"image/webp":    "webp",
}

// extensionFor returns the filename extension for a response Content-Type
// (parameters and case normalized away), or "" when the type is not a
// known image type.
func extensionFor(contentType string) string {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	return contentTypeExts[strings.ToLower(strings.TrimSpace(contentType))]
}

// imageDownloadFields are the table columns of image download; the error
// column is empty on success, so a failed row stays visible in every
// format.
var imageDownloadFields = []string{"filename", "bytes", "path", "error"}

// printImageDownloads renders one row per download under the
// one-row-vs-array output convention: successful rows carry filename,
// path, and bytes; a failed row carries filename and the named error
// instead (path and bytes are omitted, like attachment list's empty
// subtitle).
func printImageDownloads(cmd *cobra.Command, cfg *app.Config, rows []downloadRow) {
	jsonRows := make([]map[string]any, 0, len(rows))
	tableRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		jsonRow := map[string]any{"filename": row.Filename}
		if row.Err != "" {
			jsonRow["error"] = row.Err
		} else {
			jsonRow["path"] = row.Path
			jsonRow["bytes"] = row.Bytes
		}
		jsonRows = append(jsonRows, jsonRow)
		tableRows = append(tableRows, map[string]any{
			"filename": row.Filename,
			"bytes":    row.Bytes,
			"path":     row.Path,
			"error":    row.Err,
		})
	}
	output.Print(cmd.OutOrStdout(), output.ResolveOutput(cfg.Format), imageDownloadFields, jsonRows, tableRows)
}
