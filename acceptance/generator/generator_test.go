package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleIR = `{
  "name": "Chat Channel Relay",
  "background": [{"keyword": "Given", "text": "the bridge is started", "parameters": []}],
  "scenarios": [{
    "name": "Relay 1",
    "steps": [{"keyword": "Then", "text": "the forge holds <n> chat requests", "parameters": ["n"]}],
    "examples": [{"n": "1"}]
  }]
}`

func writeIR(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, "build", "acceptance", "ir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(sampleIR), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMetadataNameFollowsTheSpecMapping(t *testing.T) {
	cases := map[string]string{
		"features/Hunt The Wumpus.feature":     "features-hunt-the-wumpus-feature.json",
		"features/orders/Cancel Order.feature": "features-orders-cancel-order-feature.json",
		"Features/API v2/Happy Path.feature":   "features-api-v2-happy-path-feature.json",
	}
	for featurePath, want := range cases {
		if got := MetadataName(featurePath); got != want {
			t.Errorf("MetadataName(%q) = %q, want %q", featurePath, got, want)
		}
	}
}

func TestGenerateWritesEntryPointAndMetadata(t *testing.T) {
	root := t.TempDir()
	irPath := writeIR(t, root, "chat-channel-relay.json")
	output := filepath.Join(root, "build", "acceptance", "generated")

	metadata, err := Generate(Request{
		IRPath:      irPath,
		FeaturePath: "features/chat-channel-relay.feature",
		OutputDir:   output,
		ProjectRoot: root,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	entry := filepath.Join(output, "chat-channel-relay_acceptance_test.go")
	source, err := os.ReadFile(entry)
	if err != nil {
		t.Fatalf("read entry point: %v", err)
	}
	if !strings.Contains(string(source), "func TestChatChannelRelay(t *testing.T)") {
		t.Errorf("entry point =\n%s\nwant a test function named after the feature", source)
	}
	if !strings.Contains(string(source), "steps.Registry().Run(t, feature)") {
		t.Errorf("entry point does not delegate to the runtime and step handlers:\n%s", source)
	}
	if strings.Contains(string(source), "gherkin") || strings.Contains(string(source), "os.ReadFile(") {
		t.Errorf("entry point should not parse the feature file:\n%s", source)
	}

	if metadata.SchemaVersion != MetadataSchemaVersion {
		t.Errorf("schema version = %d", metadata.SchemaVersion)
	}
	if metadata.FeaturePath != "features/chat-channel-relay.feature" {
		t.Errorf("feature path = %q", metadata.FeaturePath)
	}
	if metadata.HashScope != "generated_files" {
		t.Errorf("hash scope = %q", metadata.HashScope)
	}
	if !strings.HasPrefix(metadata.ImplementationHash, "sha256:") || len(metadata.ImplementationHash) != len("sha256:")+64 {
		t.Errorf("implementation hash = %q", metadata.ImplementationHash)
	}
	if len(metadata.GeneratedFiles) != 1 || metadata.GeneratedFiles[0] != "build/acceptance/generated/chat-channel-relay_acceptance_test.go" {
		t.Errorf("generated files = %v", metadata.GeneratedFiles)
	}

	metadataPath := filepath.Join(output, "metadata", "features-chat-channel-relay-feature.json")
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var written Metadata
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if written.ImplementationHash != metadata.ImplementationHash {
		t.Errorf("metadata hash = %q, want %q", written.ImplementationHash, metadata.ImplementationHash)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	root := t.TempDir()
	irPath := writeIR(t, root, "chat-channel-relay.json")
	request := Request{IRPath: irPath, FeaturePath: "features/chat-channel-relay.feature", OutputDir: filepath.Join(root, "generated"), ProjectRoot: root}

	first, err := Generate(request)
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	entry := filepath.Join(request.OutputDir, "chat-channel-relay_acceptance_test.go")
	firstSource, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Generate(request)
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	secondSource, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}

	if firstSource == nil || string(firstSource) != string(secondSource) {
		t.Error("generated entry point is not deterministic")
	}
	if first.ImplementationHash != second.ImplementationHash {
		t.Errorf("implementation hashes differ: %q and %q", first.ImplementationHash, second.ImplementationHash)
	}
}

func TestGenerateChangesTheHashWithTheGeneratedFile(t *testing.T) {
	root := t.TempDir()
	irPath := writeIR(t, root, "chat-channel-relay.json")
	output := filepath.Join(root, "generated")
	request := Request{IRPath: irPath, FeaturePath: "features/chat-channel-relay.feature", OutputDir: output, ProjectRoot: root}

	first, err := Generate(request)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(output, "unrelated.go"), []byte("package generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Generate(request)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if first.ImplementationHash != second.ImplementationHash {
		t.Errorf("hash changed without the generated acceptance file changing: %q -> %q", first.ImplementationHash, second.ImplementationHash)
	}
}

func TestGenerateRejectsUnknownIR(t *testing.T) {
	if _, err := Generate(Request{IRPath: filepath.Join(t.TempDir(), "missing.json")}); err == nil {
		t.Fatal("Generate accepted a missing IR file")
	}
}
