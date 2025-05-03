package com.almyashev.speech2textbot.transcription;

import com.almyashev.speech2textbot.common.AsyncOperationService;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.io.ByteArrayResource;
import org.springframework.http.*;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.client.RestTemplate;

import java.io.InputStream;
import java.net.URL;
import java.nio.file.Paths;
import java.util.Map;
import java.util.concurrent.CompletableFuture;

@Service
@Slf4j
public class OpenAIService {

    private static final String OPENAI_URL = "https://api.openai.com/v1/audio/transcriptions";

    private final String openAIKey;
    private final RestTemplate restTemplate;
    private final AsyncOperationService asyncOperationService;

    public OpenAIService(
            @Value("${openai.key}") String openAIKey,
            AsyncOperationService asyncOperationService
    ) {
        this.openAIKey = openAIKey;
        this.restTemplate = new RestTemplate();
        this.asyncOperationService = asyncOperationService;
    }

    public CompletableFuture<String> transcribeFromUrlAsync(String fileUrl) {
        log.info("Asking OpenAI for transcription");
        return asyncOperationService.executeAsync(
                () -> doTranscribeFromUrl(fileUrl),
                "OpenAI-Transcription"
        );
    }

    @SneakyThrows
    private String doTranscribeFromUrl(String fileUrl) {
        URL url = new URL(fileUrl);
        String fileName = Paths.get(url.getPath()).getFileName().toString();
        log.debug("Processing file: {}", fileName);

        try (InputStream inputStream = url.openStream()) {
            byte[] audioBytes = inputStream.readAllBytes();

            // Создаём multipart тело
            MultiValueMap<String, Object> body = new LinkedMultiValueMap<>();
            body.add("file", new ByteArrayResource(audioBytes) {
                @Override
                public String getFilename() {
                    return fileName;
                }

                @Override
                public long contentLength() {
                    return audioBytes.length;
                }
            });
            body.add("model", "whisper-1");

            HttpHeaders headers = new HttpHeaders();
            headers.setContentType(MediaType.MULTIPART_FORM_DATA);
            headers.setBearerAuth(openAIKey);

            // Отправка запроса
            ResponseEntity<Map> response = restTemplate.postForEntity(
                    OPENAI_URL,
                    new HttpEntity<>(body, headers),
                    Map.class
            );

            if (!response.getStatusCode().is2xxSuccessful()) {
                log.error("OpenAI API error: {}", response.getStatusCode());
                throw new RuntimeException("OpenAI API error: " + response.getStatusCode());
            }

            // Возврат текста из ответа
            String text = (String) response.getBody().get("text");
            if (text == null || text.isEmpty()) {
                throw new RuntimeException("Empty response from OpenAI");
            }

            return text;
        } catch (Exception e) {
            log.error("Error during OpenAI transcription", e);
            throw e;
        }
    }
}