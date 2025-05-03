package com.almyashev.speech2textbot.transcription;

import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

@Slf4j
@Service
public class TranscribeVoiceToTextService {
    private static final int TIMEOUT_SECONDS = 300;
    private final AssemblyAIService assemblyAIService;
    private final OpenAIService openAIService;
    private final boolean useOpenAI;

    public TranscribeVoiceToTextService(
            AssemblyAIService assemblyAIService,
            OpenAIService openAIService,
            @Value("${use.openai}") boolean useOpenAI
    ) {
        this.assemblyAIService = assemblyAIService;
        this.openAIService = openAIService;
        this.useOpenAI = useOpenAI;
    }

    public String transcribe(String audioFileURL) {
        try {
            CompletableFuture<String> future = useOpenAI 
                ? openAIService.transcribeFromUrlAsync(audioFileURL)
                : assemblyAIService.transcribeFromUrlAsync(audioFileURL);
                
            return future.get(TIMEOUT_SECONDS, TimeUnit.SECONDS);
        } catch (TimeoutException e) {
            log.error("Transcription timed out after {} seconds", TIMEOUT_SECONDS, e);
            throw new RuntimeException("Transcription timed out", e);
        } catch (InterruptedException e) {
            log.error("Transcription was interrupted", e);
            Thread.currentThread().interrupt();
            throw new RuntimeException("Transcription was interrupted", e);
        } catch (ExecutionException e) {
            log.error("Transcription failed", e.getCause());
            throw new RuntimeException("Transcription failed", e.getCause());
        }
    }
}
