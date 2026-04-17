package providers

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/parlona/cloudapp/pkg/contracts"
)

// GRPCClientConfig contains configuration for gRPC provider clients.
type GRPCClientConfig struct {
	Address    string
	Timeout    int // seconds
	MaxRetries int
}

// Validate validates the gRPC client configuration.
func (c *GRPCClientConfig) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("gRPC address is required")
	}
	if c.Timeout <= 0 {
		c.Timeout = 30
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	return nil
}

// GRPCASRProvider implements ASRProvider using gRPC.
// This is a stub implementation showing the structure for connecting to the Python provider-gateway.
type GRPCASRProvider struct {
	name   string
	config GRPCClientConfig
	conn   *grpc.ClientConn
	// TODO: Add generated gRPC client when proto generation is set up
}

// NewGRPCASRProvider creates a new gRPC ASR provider.
func NewGRPCASRProvider(name string, config GRPCClientConfig) (*GRPCASRProvider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	conn, err := grpc.Dial(config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &GRPCASRProvider{
		name:   name,
		config: config,
		conn:   conn,
	}, nil
}

// StreamRecognize performs streaming speech recognition via gRPC.
func (p *GRPCASRProvider) StreamRecognize(ctx context.Context, audioStream chan []byte, opts ASROptions) (chan ASRResult, error) {
	resultCh := make(chan ASRResult, 10)

	// TODO: Implement using generated gRPC client
	// This is a stub showing the expected structure
	go func() {
		defer close(resultCh)

		// Stub: Echo back partial results
		for range audioStream {
			select {
			case <-ctx.Done():
				return
			case resultCh <- ASRResult{
				Transcript: "[stub]",
				IsPartial:  true,
			}:
			}
		}

		// Send final result
		select {
		case <-ctx.Done():
			return
		case resultCh <- ASRResult{
			Transcript: "[stub final]",
			IsFinal:    true,
		}:
		}
	}()

	return resultCh, nil
}

// Cancel cancels an ongoing recognition.
func (p *GRPCASRProvider) Cancel(ctx context.Context, sessionID string) error {
	// TODO: Implement using generated gRPC client
	return fmt.Errorf("GRPCASRProvider.Cancel not implemented")
}

// Capabilities returns the provider's capabilities.
func (p *GRPCASRProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{
		SupportsStreamingInput:  true,
		SupportsStreamingOutput: true,
		SupportsWordTimestamps:  false,
		PreferredSampleRates:    []int32{16000},
		SupportedCodecs:         []string{"pcm16"},
	}
}

// Name returns the provider name.
func (p *GRPCASRProvider) Name() string {
	return p.name
}

// Close closes the gRPC connection.
func (p *GRPCASRProvider) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// GRPCLLMProvider implements LLMProvider using gRPC.
type GRPCLLMProvider struct {
	name   string
	config GRPCClientConfig
	conn   *grpc.ClientConn
	// TODO: Add generated gRPC client when proto generation is set up
}

// NewGRPCLLMProvider creates a new gRPC LLM provider.
func NewGRPCLLMProvider(name string, config GRPCClientConfig) (*GRPCLLMProvider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	conn, err := grpc.Dial(config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &GRPCLLMProvider{
		name:   name,
		config: config,
		conn:   conn,
	}, nil
}

// StreamGenerate performs streaming text generation via gRPC.
func (p *GRPCLLMProvider) StreamGenerate(ctx context.Context, messages []contracts.ChatMessage, opts LLMOptions) (chan LLMToken, error) {
	tokenCh := make(chan LLMToken, 10)

	// TODO: Implement using generated gRPC client
	go func() {
		defer close(tokenCh)

		// Stub: Send a completion token
		select {
		case <-ctx.Done():
			return
		case tokenCh <- LLMToken{
			Token:   "[stub response]",
			IsFinal: true,
		}:
		}
	}()

	return tokenCh, nil
}

// Cancel cancels an ongoing generation.
func (p *GRPCLLMProvider) Cancel(ctx context.Context, sessionID string) error {
	// TODO: Implement using generated gRPC client
	return fmt.Errorf("GRPCLLMProvider.Cancel not implemented")
}

// Capabilities returns the provider's capabilities.
func (p *GRPCLLMProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{
		SupportsStreamingInput:          true,
		SupportsStreamingOutput:         true,
		SupportsInterruptibleGeneration: true,
	}
}

// Name returns the provider name.
func (p *GRPCLLMProvider) Name() string {
	return p.name
}

// Close closes the gRPC connection.
func (p *GRPCLLMProvider) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// GRPCTTSProvider implements TTSProvider using gRPC.
type GRPCTTSProvider struct {
	name   string
	config GRPCClientConfig
	conn   *grpc.ClientConn
	// TODO: Add generated gRPC client when proto generation is set up
}

// NewGRPCTTSProvider creates a new gRPC TTS provider.
func NewGRPCTTSProvider(name string, config GRPCClientConfig) (*GRPCTTSProvider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	conn, err := grpc.Dial(config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return &GRPCTTSProvider{
		name:   name,
		config: config,
		conn:   conn,
	}, nil
}

// StreamSynthesize performs streaming text-to-speech via gRPC.
func (p *GRPCTTSProvider) StreamSynthesize(ctx context.Context, text string, opts TTSOptions) (chan []byte, error) {
	audioCh := make(chan []byte, 10)

	// TODO: Implement using generated gRPC client
	go func() {
		defer close(audioCh)

		// Stub: Send silence (PCM16 zeros)
		silence := make([]byte, 320) // 10ms of silence at 16kHz
		select {
		case <-ctx.Done():
			return
		case audioCh <- silence:
		}
	}()

	return audioCh, nil
}

// Cancel cancels an ongoing synthesis.
func (p *GRPCTTSProvider) Cancel(ctx context.Context, sessionID string) error {
	// TODO: Implement using generated gRPC client
	return fmt.Errorf("GRPCTTSProvider.Cancel not implemented")
}

// Capabilities returns the provider's capabilities.
func (p *GRPCTTSProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{
		SupportsStreamingInput:  true,
		SupportsStreamingOutput: true,
		SupportsVoices:          true,
		PreferredSampleRates:    []int32{16000, 22050, 24000},
		SupportedCodecs:         []string{"pcm16", "opus"},
	}
}

// Name returns the provider name.
func (p *GRPCTTSProvider) Name() string {
	return p.name
}

// Close closes the gRPC connection.
func (p *GRPCTTSProvider) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// Ensure interfaces are implemented
var (
	_ ASRProvider = (*GRPCASRProvider)(nil)
	_ LLMProvider = (*GRPCLLMProvider)(nil)
	_ TTSProvider = (*GRPCTTSProvider)(nil)
)

// io.EOF import check
var _ = io.EOF
