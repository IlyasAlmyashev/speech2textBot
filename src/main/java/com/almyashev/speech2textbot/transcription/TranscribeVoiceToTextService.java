package com.almyashev.speech2textbot.transcription;

import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@AllArgsConstructor
public class TranscribeVoiceToTextService {
    private final AssemblyAIClient client;

    public String transcribe(String audioFileURL) {
        return client.createTranscription(audioFileURL);
    }
}
