package com.almyashev.speech2textbot.telegram;

import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.io.IOUtils;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.telegram.telegrambots.bots.DefaultAbsSender;
import org.telegram.telegrambots.meta.api.methods.GetFile;
import org.telegram.telegrambots.meta.api.objects.File;

import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.net.URL;

@Slf4j
@Service
public class TelegramFileService {

    private final DefaultAbsSender telegramSender;
    private final String botToken;

    public TelegramFileService(
            @Lazy DefaultAbsSender telegramSender,
            @Value("${bot.token}") String botToken
    ) {
        this.telegramSender = telegramSender;
        this.botToken = botToken;
    }

    @SneakyThrows
    public java.io.File getFile(String fileId) {
        log.info("Getting the file from telegram, fileId={}", fileId);
        File file = telegramSender.execute(GetFile.builder()
                .fileId(fileId)
                .build());
        return getFileFromUrl(file.getFileUrl(botToken));
    }

    @SneakyThrows
    private java.io.File getFileFromUrl(String urlToDownloadFile) {
        URL url = new URI(urlToDownloadFile).toURL();
        var fileTemp = java.io.File.createTempFile("telegram", ".ogg");

        try (InputStream inputStream = url.openStream();
             FileOutputStream fileOutputStream = new FileOutputStream(fileTemp)
        ) {
            IOUtils.copy(inputStream, fileOutputStream);
        } catch (IOException e) {
            log.error("Ошибка во время загрузки файла", e);
        }

        return fileTemp;
    }
}
