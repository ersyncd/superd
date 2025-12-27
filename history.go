package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func (a *App) SaveTransaction(ops []MoveOperation) {
	path := filepath.Join(a.GetAppDir(), "history.json")
	var h []Transaction
	d, _ := os.ReadFile(path)
	json.Unmarshal(d, &h)

	tx := Transaction{
		ID:         time.Now().Format("15:04:05"),
		Timestamp:  time.Now().Unix(),
		Operations: ops,
	}
	h = append([]Transaction{tx}, h...)
	if len(h) > 30 {
		h = h[:30]
	}
	out, _ := json.MarshalIndent(h, "", "  ")
	os.WriteFile(path, out, 0644)
}

func (a *App) GetHistory() []Transaction {
	path := filepath.Join(a.GetAppDir(), "history.json")
	d, err := os.ReadFile(path)
	if err != nil {
		return []Transaction{}
	}
	var h []Transaction
	json.Unmarshal(d, &h)
	return h
}

func (a *App) UndoByID(id string) error {
	history := a.GetHistory()
	for i, tx := range history {
		if tx.ID == id {
			for j := len(tx.Operations) - 1; j >= 0; j-- {
				op := tx.Operations[j]
				os.MkdirAll(filepath.Dir(op.OldPath), 0755)
				os.Rename(op.NewPath, op.OldPath)
			}
			history = append(history[:i], history[i+1:]...)
			out, _ := json.MarshalIndent(history, "", "  ")
			os.WriteFile(filepath.Join(a.GetAppDir(), "history.json"), out, 0644)
			return nil
		}
	}
	return nil
}
