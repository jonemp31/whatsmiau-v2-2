package whatsmiau

import (
	"encoding/base64"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/verbeux-ai/whatsmiau/models"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

func b64(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func u64(n uint64) string {
	return strconv.FormatUint(n, 10)
}

func i64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func extractMimetype(decodedData []byte, fileName string) (string, error) {
	ext := filepath.Ext(fileName)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType, nil
		}
	}

	var dataSample []byte
	if len(decodedData) > 512 {
		dataSample = decodedData[:512]
	} else {
		dataSample = decodedData
	}
	detected := http.DetectContentType(dataSample)
	return detected, nil
}

func extractExtFromFile(fileName, mimeType string, file *os.File) string {
	ext := filepath.Ext(fileName)
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			if len(exts) > 1 {
				ext = exts[1]
			} else {
				ext = exts[0]
			}
		} else {
			buf := make([]byte, 512)
			n, err := file.Read(buf)
			if err != nil && err != io.EOF {
				zap.L().Error("failed to read file", zap.Error(err))
			}
			detected := http.DetectContentType(buf[:n])
			if exts, _ := mime.ExtensionsByType(detected); len(exts) > 0 {
				ext = exts[0]
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				zap.L().Error("failed to seek image", zap.Error(err))
			}
		}
	}

	return strings.TrimPrefix(ext, ".")
}

// canIgnoreGroup returns true if group can be ignored
func canIgnoreGroup(evt interface{}, instance *models.Instance) bool {
	if !instance.GroupsIgnore {
		return false
	}

	var jid string
	switch evt.(type) {
	case *events.Message:
		msg, ok := evt.(*events.Message)
		if !ok {
			return false
		}

		jid = msg.Info.Chat.String()
	case *events.GroupInfo:
		gInfo, ok := evt.(*events.GroupInfo)
		if !ok {
			return false
		}

		jid = gInfo.JID.String()
	case *events.Receipt:
		rcp, ok := evt.(*events.Receipt)
		if !ok {
			return false
		}

		jid = rcp.Chat.String()
	case *events.Contact:
		ctc, ok := evt.(*events.Contact)
		if !ok {
			return false
		}

		jid = ctc.JID.String()
	case *events.Picture:
		pic, ok := evt.(*events.Picture)
		if !ok {
			return false
		}

		jid = pic.JID.String()
	case *events.PushName:
		pushName, ok := evt.(*events.PushName)
		if !ok {
			return false
		}

		jid = pushName.JID.String()
	}

	return strings.HasSuffix(jid, "@g.us")
}
