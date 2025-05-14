package com.almyashev.speech2textbot.telegram;

import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Lazy;
import org.springframework.stereotype.Service;
import org.telegram.telegrambots.bots.DefaultAbsSender;
import org.telegram.telegrambots.meta.api.methods.send.SendMessage;
import org.telegram.telegrambots.meta.api.methods.updatingmessages.EditMessageText;
import org.telegram.telegrambots.meta.exceptions.TelegramApiException;

import com.almyashev.speech2textbot.common.AsyncOperationService;

import java.util.function.Function;
import java.util.function.Supplier;

@Slf4j
@Service
public class TelegramAsyncMessageSender {
    private static final String WAIT_MESSAGE = "Your request has been accepted for processing, please wait.";

    private final DefaultAbsSender defaultAbsSender;
    private final AsyncOperationService asyncOperationService;

    public TelegramAsyncMessageSender(
            @Lazy DefaultAbsSender defaultAbsSender,
            AsyncOperationService asyncOperationService
    ) {
        this.defaultAbsSender = defaultAbsSender;
        this.asyncOperationService = asyncOperationService;
    }

    @SneakyThrows
    public void sendMessageAsync(
            String chatId,
            Supplier<SendMessage> action,
            Function<Throwable, SendMessage> onErrorHandler
    ) {
        log.info("Send message async: chatId={}", chatId);
        var message = defaultAbsSender.execute(SendMessage.builder()
                .text(WAIT_MESSAGE)
                .chatId(chatId)
                .build());

        asyncOperationService.executeAsync(action, "Telegram-Message")
                .exceptionally(onErrorHandler)
                .thenAccept(sendMessage -> updateMessage(chatId, message.getMessageId(), sendMessage));
    }

    private void updateMessage(String chatId, Integer messageId, SendMessage sendMessage) {
        try {
            log.info("Send edit message async: chatId={}", chatId);
            defaultAbsSender.execute(EditMessageText.builder()
                    .chatId(chatId)
                    .messageId(messageId)
                    .text(sendMessage.getText())
                    .build());
        } catch (TelegramApiException e) {
            log.error("Error while send request to telegram", e);
            throw new RuntimeException(e);
        }
    }
}
