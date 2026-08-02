package mcpserver

// ReadPageOutputSchemaForTest exposes kb_read's declared output schema so a
// test in the external mcpserver_test package can assert it stays strict —
// see TestKBRead_MediaMetaDoesNotChangeStructuredContent for why that matters.
func ReadPageOutputSchemaForTest() map[string]any { return readPageOutputSchema() }
