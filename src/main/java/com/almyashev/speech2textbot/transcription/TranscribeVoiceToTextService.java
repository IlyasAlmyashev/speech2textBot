package com.almyashev.speech2textbot.transcription;

import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;

import java.io.File;

@Service
@AllArgsConstructor
public class TranscribeVoiceToTextService {
    private final AssemblyAIClient client;

    public String transcribe(File audioFile) {
        return client.createTranscription(audioFile);
    }

    public String transcribe(String audioFileURL) {
        return client.createTranscription(audioFileURL);
    }
}
