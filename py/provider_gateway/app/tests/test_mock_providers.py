"""Tests for mock providers."""

import asyncio
import pytest
from app.providers.asr.mock_asr import MockASRProvider, create_mock_asr_provider
from app.providers.llm.mock_llm import MockLLMProvider, create_mock_llm_provider
from app.providers.tts.mock_tts import MockTTSProvider, create_mock_tts_provider
from app.models.asr import ASROptions
from app.models.llm import LLMOptions, ChatMessage
from app.models.tts import TTSOptions


class TestMockASRProvider:
    """Test cases for MockASRProvider."""

    @pytest.fixture
    def provider(self):
        """Create a mock ASR provider."""
        return MockASRProvider(chunk_delay_ms=10)  # Fast for tests

    @pytest.fixture
    def audio_stream(self):
        """Create a mock audio stream."""
        async def stream():
            # Yield some dummy audio chunks
            for _ in range(3):
                yield b"\x00\x01\x02\x03"
        return stream()

    @pytest.mark.asyncio
    async def test_mock_asr_produces_transcript(self, provider, audio_stream):
        """Test that mock ASR produces transcript responses."""
        options = ASROptions(language_hint="en-US")

        responses = []
        async for response in provider.stream_recognize(audio_stream, options):
            responses.append(response)

        # Should have partial transcripts + final transcript
        assert len(responses) > 0

        # Last response should be final
        assert responses[-1].is_final is True
        assert responses[-1].transcript != ""

        # Should have transcript content
        assert len(responses[-1].transcript) > 0

    @pytest.mark.asyncio
    async def test_mock_asr_partial_results(self, provider, audio_stream):
        """Test that mock ASR produces partial results."""
        options = ASROptions(language_hint="en-US")

        partial_count = 0
        async for response in provider.stream_recognize(audio_stream, options):
            if response.is_partial:
                partial_count += 1
            if response.is_final:
                break

        # Should have some partial results
        assert partial_count > 0

    @pytest.mark.asyncio
    async def test_mock_asr_word_timestamps(self, provider, audio_stream):
        """Test that final result has word timestamps."""
        options = ASROptions(language_hint="en-US")

        async for response in provider.stream_recognize(audio_stream, options):
            if response.is_final:
                # Should have word timestamps
                assert len(response.word_timestamps) > 0
                # Each timestamp should have word, start_ms, end_ms
                for wt in response.word_timestamps:
                    assert wt.word != ""
                    assert wt.start_ms >= 0
                    assert wt.end_ms > wt.start_ms

    @pytest.mark.asyncio
    async def test_mock_asr_cancellation(self, provider):
        """Test ASR cancellation during streaming."""
        async def slow_stream():
            for i in range(100):
                yield b"\x00\x01\x02\x03"
                await asyncio.sleep(0.01)

        options = ASROptions(language_hint="en-US")

        # Start recognition
        stream = provider.stream_recognize(slow_stream(), options)

        # Cancel after a short delay
        await asyncio.sleep(0.05)
        await provider.cancel("test_session")

        # Collect remaining responses
        responses = []
        async for response in stream:
            responses.append(response)

        # Should have stopped early due to cancellation
        # (may have some responses before cancellation took effect)

    @pytest.mark.asyncio
    async def test_mock_asr_custom_transcript(self):
        """Test ASR with custom transcript."""
        custom_text = "This is a custom transcript for testing."
        provider = MockASRProvider(transcript=custom_text, chunk_delay_ms=1)

        async def stream():
            yield b"audio"

        options = ASROptions(language_hint="en-US")

        async for response in provider.stream_recognize(stream(), options):
            if response.is_final:
                assert response.transcript == custom_text

    def test_mock_asr_capabilities(self, provider):
        """Test ASR provider capabilities."""
        caps = provider.capabilities()

        assert caps.supports_streaming_input is True
        assert caps.supports_streaming_output is True
        assert caps.supports_word_timestamps is True
        assert caps.supports_interruptible_generation is True
        assert 16000 in caps.preferred_sample_rates
        assert "pcm16" in caps.supported_codecs

    def test_mock_asr_name(self, provider):
        """Test ASR provider name."""
        assert provider.name() == "mock_asr"


class TestMockLLMProvider:
    """Test cases for MockLLMProvider."""

    @pytest.fixture
    def provider(self):
        """Create a mock LLM provider."""
        return MockLLMProvider(token_delay_ms=1)  # Fast for tests

    @pytest.fixture
    def messages(self):
        """Create test messages."""
        return [
            ChatMessage(role="system", content="You are helpful."),
            ChatMessage(role="user", content="Hello!"),
        ]

    @pytest.mark.asyncio
    async def test_mock_llm_streams_tokens(self, provider, messages):
        """Test that mock LLM streams tokens."""
        options = LLMOptions(max_tokens=100)

        tokens = []
        async for response in provider.stream_generate(messages, options):
            tokens.append(response.token)
            if response.is_final:
                break

        # Should have streamed some tokens
        assert len(tokens) > 0

        # Combined tokens should form the response
        full_response = "".join(tokens)
        assert len(full_response) > 0

    @pytest.mark.asyncio
    async def test_mock_llm_final_response(self, provider, messages):
        """Test that mock LLM produces final response with metadata."""
        options = LLMOptions(max_tokens=100)

        final_response = None
        async for response in provider.stream_generate(messages, options):
            if response.is_final:
                final_response = response
                break

        assert final_response is not None
        assert final_response.finish_reason == "stop"
        assert final_response.usage is not None
        assert final_response.usage.prompt_tokens > 0
        assert final_response.usage.completion_tokens > 0
        assert final_response.usage.total_tokens > 0

    @pytest.mark.asyncio
    async def test_mock_llm_cancellation(self, provider, messages):
        """Test LLM cancellation during generation."""
        options = LLMOptions(max_tokens=1000)

        # Start generation
        stream = provider.stream_generate(messages, options)

        # Cancel after a short delay
        await asyncio.sleep(0.01)

        # Generate a session ID from messages (same logic as provider)
        import hashlib
        message_hash = hashlib.md5(
            "".join(m.content for m in messages).encode()
        ).hexdigest()[:16]
        session_id = f"mock_{message_hash}"

        await provider.cancel(session_id)

        # Collect remaining responses
        tokens = []
        async for response in stream:
            tokens.append(response)

        # Should have stopped early
        assert len(tokens) < 100  # Should be much less than full generation

    @pytest.mark.asyncio
    async def test_mock_llm_custom_response(self, messages):
        """Test LLM with custom response text."""
        custom_text = "This is a custom LLM response."
        provider = MockLLMProvider(response_text=custom_text, token_delay_ms=1)

        options = LLMOptions(max_tokens=100)

        tokens = []
        async for response in provider.stream_generate(messages, options):
            tokens.append(response.token)
            if response.is_final:
                break

        full_response = "".join(tokens)
        assert full_response == custom_text

    def test_mock_llm_capabilities(self, provider):
        """Test LLM provider capabilities."""
        caps = provider.capabilities()

        assert caps.supports_streaming_input is False
        assert caps.supports_streaming_output is True
        assert caps.supports_interruptible_generation is True

    def test_mock_llm_name(self, provider):
        """Test LLM provider name."""
        assert provider.name() == "mock_llm"


class TestMockTTSProvider:
    """Test cases for MockTTSProvider."""

    @pytest.fixture
    def provider(self):
        """Create a mock TTS provider."""
        return MockTTSProvider(chunk_delay_ms=1)  # Fast for tests

    @pytest.mark.asyncio
    async def test_mock_tts_produces_audio(self, provider):
        """Test that mock TTS produces PCM16 audio chunks."""
        text = "Hello, this is a test."
        options = TTSOptions(voice_id="default")

        chunks = []
        async for response in provider.stream_synthesize(text, options):
            chunks.append(response.audio_chunk)
            if response.is_final:
                break

        # Should have produced audio chunks
        assert len(chunks) > 0

        # All chunks except possibly the last should have audio data
        total_audio = sum(len(chunk) for chunk in chunks[:-1])
        assert total_audio > 0

    @pytest.mark.asyncio
    async def test_mock_tts_audio_format(self, provider):
        """Test that TTS audio is in correct format."""
        text = "Test."
        options = TTSOptions(voice_id="default")

        async for response in provider.stream_synthesize(text, options):
            if response.audio_format:
                assert response.audio_format.sample_rate == 16000
                assert response.audio_format.channels == 1

    @pytest.mark.asyncio
    async def test_mock_tts_segment_indexing(self, provider):
        """Test that TTS chunks have correct segment indices."""
        text = "This is a longer text that should produce multiple chunks."
        options = TTSOptions(voice_id="default")

        indices = []
        async for response in provider.stream_synthesize(text, options):
            indices.append(response.segment_index)
            if response.is_final:
                break

        # Indices should be sequential starting from 0
        for i, idx in enumerate(indices):
            assert idx == i

    @pytest.mark.asyncio
    async def test_mock_tts_final_chunk(self, provider):
        """Test that TTS produces a final chunk."""
        text = "Test."
        options = TTSOptions(voice_id="default")

        found_final = False
        async for response in provider.stream_synthesize(text, options):
            if response.is_final:
                found_final = True
                break

        assert found_final is True

    @pytest.mark.asyncio
    async def test_mock_tts_cancellation(self, provider):
        """Test TTS cancellation during synthesis."""
        text = "This is a long text that would normally produce many chunks."
        options = TTSOptions(voice_id="default")

        # Start synthesis
        stream = provider.stream_synthesize(text, options)

        # Cancel after a short delay
        await asyncio.sleep(0.01)

        # Generate session ID from text (same logic as provider)
        import hashlib
        text_hash = hashlib.md5(text.encode()).hexdigest()[:16]
        session_id = f"mock_tts_{text_hash}"

        await provider.cancel(session_id)

        # Collect remaining responses
        chunks = []
        async for response in stream:
            chunks.append(response)

        # Should have stopped early
        assert len(chunks) < 50  # Should be much less than full synthesis

    def test_mock_tts_capabilities(self, provider):
        """Test TTS provider capabilities."""
        caps = provider.capabilities()

        assert caps.supports_streaming_input is False
        assert caps.supports_streaming_output is True
        assert caps.supports_voices is True
        assert caps.supports_interruptible_generation is True
        assert 16000 in caps.preferred_sample_rates
        assert "pcm16" in caps.supported_codecs

    def test_mock_tts_name(self, provider):
        """Test TTS provider name."""
        assert provider.name() == "mock_tts"


class TestProviderFactories:
    """Test cases for provider factory functions."""

    def test_create_mock_asr_provider(self):
        """Test ASR provider factory."""
        provider = create_mock_asr_provider(transcript="Custom")
        assert isinstance(provider, MockASRProvider)

    def test_create_mock_llm_provider(self):
        """Test LLM provider factory."""
        provider = create_mock_llm_provider(response_text="Custom")
        assert isinstance(provider, MockLLMProvider)

    def test_create_mock_tts_provider(self):
        """Test TTS provider factory."""
        provider = create_mock_tts_provider(frequency=880.0)
        assert isinstance(provider, MockTTSProvider)
