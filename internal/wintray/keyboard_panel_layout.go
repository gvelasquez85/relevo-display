package wintray

import (
	"image"
	"relevo/internal/remoteinput"
	"strings"
)

type panelKey struct {
	Bounds      image.Rectangle
	Label, Code string
}

func keyboardPanelLayout(width, height int) []panelKey {
	rows := [][]string{{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "¿", "?", "Cerrar"}, {"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "Borrar"}, {"a", "s", "d", "f", "g", "h", "j", "k", "l", "ñ", "Intro"}, {"Mayús", "z", "x", "c", "v", "b", "n", "m", ",", ".", "-"}, {"Tab", "á", "é", "í", "ó", "ú", "Espacio", "←", "↑", "↓", "→"}}
	codes := map[string]string{"Cerrar": "close", "Mayús": "shift", "Borrar": "Backspace", "Intro": "Enter", "Tab": "Tab", "Espacio": "Space", "←": "ArrowLeft", "→": "ArrowRight", "↑": "ArrowUp", "↓": "ArrowDown"}
	var out []panelKey
	for r, row := range rows {
		for c, label := range row {
			out = append(out, panelKey{image.Rect(c*width/len(row)+3, r*height/len(rows)+3, (c+1)*width/len(row)-3, (r+1)*height/len(rows)-3), label, codes[label]})
		}
	}
	return out
}
func keyboardPanelHit(keys []panelKey, x, y int) (panelKey, bool) {
	for _, k := range keys {
		if image.Pt(x, y).In(k.Bounds) {
			return k, true
		}
	}
	return panelKey{}, false
}
func (k panelKey) event(shift bool) remoteinput.Key {
	if k.Code != "" {
		return remoteinput.Key{Kind: "press", Code: k.Code}
	}
	text := k.Label
	if shift {
		text = strings.ToUpper(text)
		if symbol, ok := map[string]string{"1": "!", "2": "@", "3": "#", "4": "$", "5": "%", "6": "&", "7": "/", "8": "(", "9": ")", "0": "=", ",": ";", ".": ":", "-": "_", "¿": "¡", "?": "!"}[k.Label]; ok {
			text = symbol
		}
	}
	return remoteinput.Key{Kind: "text", Text: text}
}
