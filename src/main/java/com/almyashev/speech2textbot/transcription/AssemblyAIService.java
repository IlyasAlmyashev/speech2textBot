package com.almyashev.speech2textbot.transcription;

import com.almyashev.speech2textbot.common.AsyncOperationService;
import com.assemblyai.api.AssemblyAI;
import com.assemblyai.api.core.RequestOptions;
import com.assemblyai.api.resources.transcripts.types.TranscriptOptionalParams;
import com.assemblyai.api.resources.transcripts.types.TranscriptStatus;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

@Service
@Slf4j
public final class AssemblyAIService {
    private static final int TIMEOUT = 300;

    private final AssemblyAI client;
    private final AsyncOperationService asyncOperationService;

    public AssemblyAIService(
            @Value("${assembly.key}") String assemblyAPIKey,
            AsyncOperationService asyncOperationService
    ) {
        this.client = AssemblyAI.builder()
                .apiKey(assemblyAPIKey)
                .build();
        this.asyncOperationService = asyncOperationService;
    }

    public CompletableFuture<String> transcribeFromUrlAsync(String fileURL) {
        log.info("Asking AssemblyAI for transcription");
        return asyncOperationService.executeAsync(
                () -> doTranscribeFromUrl(fileURL),
                "AssemblyAI-Transcription"
        );
    }

    private String doTranscribeFromUrl(String fileURL) {
        var params = TranscriptOptionalParams.builder()
                .languageDetection(true)
                .build();

        var transcriptId = client
                .transcripts()
                .transcribe(fileURL, params)
                .getId();

        var transcript = client
                .transcripts()
                .get(
                        transcriptId,
                        RequestOptions.builder()
                                .timeout(TIMEOUT, TimeUnit.SECONDS)
                                .build()
                );

        if (transcript.getStatus() == TranscriptStatus.ERROR) {
            log.error("Transcript failed with error: {}", transcript.getError().orElse(null));
            throw new RuntimeException(transcript.getError().orElse("Unknown error"));
        }

        return transcript.getText().orElse(null);
    }
}
