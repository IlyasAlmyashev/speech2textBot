package com.almyashev.speech2textbot.transcription;

import com.assemblyai.api.AssemblyAI;
import com.assemblyai.api.resources.transcripts.types.*;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.File;

@Service
@Slf4j
public final class AssemblyAIClient {
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

        Transcript transcript = client.transcripts().transcribe(file, params);

        if (transcript.getStatus() == TranscriptStatus.ERROR) {
            log.error("Transcript failed with error: " + transcript.getError().get());
        }

        return transcript.getText().get();
    }
}

