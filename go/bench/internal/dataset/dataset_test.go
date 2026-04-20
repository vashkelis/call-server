package dataset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSilence(t *testing.T) {
	silence := GenerateSilence(1000, 16000)
	expectedLen := 16000 * 2
	if len(silence) != expectedLen {
		t.Errorf("expected %d bytes, got %d", expectedLen, len(silence))
	}
	for _, b := range silence {
		if b != 0 {
			t.Error("expected all zero bytes in silence")
			break
		}
	}
}

func TestLoadWAVFile(t *testing.T) {
	audioPath := filepath.Join("..", "..", "samples", "test_audio.wav")
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		t.Skip("test_audio.wav not found")
	}
	audio, err := LoadWAVFile(audioPath)
	if err != nil {
		t.Fatalf("failed to load WAV: %v", err)
	}
	if audio.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", audio.SampleRate)
	}
	if audio.Channels != 1 {
		t.Errorf("expected 1 channel, got %d", audio.Channels)
	}
	if len(audio.Data) == 0 {
		t.Error("expected non-empty audio data")
	}
}

func TestLoadPromptsFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "prompts.txt")
	content := "# Comment\nHello world\nHow are you?\n\nGoodbye\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	prompts, err := LoadPromptsFile(path)
	if err != nil {
		t.Fatalf("failed to load prompts: %v", err)
	}
	if len(prompts) != 3 {
		t.Errorf("expected 3 prompts, got %d", len(prompts))
	}
	if prompts[0].Text != "Hello world" {
		t.Errorf("expected first prompt 'Hello world', got %q", prompts[0].Text)
	}
}

func TestLoadTextsFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "texts.txt")
	content := "Hello there\n# Skip this\nNice to meet you\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	texts, err := LoadTextsFile(path)
	if err != nil {
		t.Fatalf("failed to load texts: %v", err)
	}
	if len(texts) != 2 {
		t.Errorf("expected 2 texts, got %d", len(texts))
	}
}

func TestLoadPromptsFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(path, []byte("# Only comments\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPromptsFile(path)
	if err == nil {
		t.Error("expected error for empty prompts file")
	}
}
