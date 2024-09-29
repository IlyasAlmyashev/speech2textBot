package com.almyashev.speech2textbot.transcription;

import com.assemblyai.api.AssemblyAI;
import com.assemblyai.api.core.RequestOptions;
import com.assemblyai.api.resources.transcripts.types.TranscriptOptionalParams;
import com.assemblyai.api.resources.transcripts.types.TranscriptStatus;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.File;
import java.util.concurrent.TimeUnit;

@Service
@Slf4j
public final class AssemblyAIClient {
    private static final int TIMEOUT = 180;
    private final String assemblyKey;

    public AssemblyAIClient(
            @Value("${assembly.key}") String assemblyKey
    ) {
        this.assemblyKey = assemblyKey;
    }

    @SneakyThrows
    public String createTranscription(File file) {
        log.info("Asking AssemblyAI for transcription");
        AssemblyAI client = AssemblyAI.builder()
                .apiKey(assemblyKey)
                .build();
        var params = TranscriptOptionalParams.builder()
                .languageDetection(true)
                .build();
        var transcriptId = client
                .transcripts()
                .transcribe(file, params)
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
            log.error("Transcript failed with error: " + transcript.getError().orElse(null));
        }

        return transcript.getText().orElse(null);
    }

    @SneakyThrows
    public String createTranscription(String fileURL) {
        log.info("Asking AssemblyAI for transcription");
        AssemblyAI client = AssemblyAI.builder()
                .apiKey(assemblyKey)
                .build();
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
            log.error("Transcript failed with error: " + transcript.getError().orElse(null));
            return transcript.getError().orElse(null);
        }

        return transcript.getText().orElse(null);
    }
}
