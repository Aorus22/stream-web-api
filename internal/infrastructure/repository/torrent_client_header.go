package repository

import (
	"fmt"

	"github.com/anacrolix/torrent"
)

func (c *Client) EnsureFileHeader(infoHashHex string, fileIndex int) error {
	t := c.GetTorrent(infoHashHex)
	if t == nil {
		return fmt.Errorf("torrent not found")
	}
	if t.Info() == nil {
		return fmt.Errorf("no metadata")
	}
	files := t.Files()
	if fileIndex < 0 || fileIndex >= len(files) {
		return fmt.Errorf("invalid file index")
	}
	file := files[fileIndex]
	pieceLength := t.Info().PieceLength

	const HeaderSize = 10 * 1024 * 1024
	headerEnd := file.Offset() + HeaderSize
	headerEndPiece := int(headerEnd / pieceLength)
	startPiece := int(file.Offset() / pieceLength)

	if headerEndPiece >= t.NumPieces() {
		headerEndPiece = t.NumPieces() - 1
	}

	for i := startPiece; i <= headerEndPiece; i++ {
		if !t.Piece(i).State().Complete {
			t.Piece(i).SetPriority(torrent.PiecePriorityHigh)
		}
	}
	return nil
}
