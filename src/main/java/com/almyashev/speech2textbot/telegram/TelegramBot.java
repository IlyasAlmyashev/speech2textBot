package com.almyashev.speech2textbot.telegram;

import jakarta.annotation.PostConstruct;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.telegram.telegrambots.bots.DefaultBotOptions;
import org.telegram.telegrambots.bots.TelegramWebhookBot;
import org.telegram.telegrambots.meta.api.methods.BotApiMethod;
import org.telegram.telegrambots.meta.api.methods.send.SendMessage;
import org.telegram.telegrambots.meta.api.methods.updates.SetWebhook;
import org.telegram.telegrambots.meta.api.objects.Update;
import org.telegram.telegrambots.meta.exceptions.TelegramApiException;

@Slf4j
@Component
public class TelegramBot extends TelegramWebhookBot {
    @Value("${bot.url}")
    private String botUrl;
    private final UpdateProcessor updateProcessor;

    public TelegramBot(
            @Value("${bot.token}") String botToken,
            UpdateProcessor updateProcessor
    ) {
        super(new DefaultBotOptions(), botToken);
        this.updateProcessor = updateProcessor;
    }

    @PostConstruct
    public void init() {
        try {
            this.setWebhook(SetWebhook.builder()
                    .url(botUrl)
                    .build());
        } catch (TelegramApiException e) {
            log.error("Ошибка инициализации вебхука: ", e);
        }
    }

    @Override
    public BotApiMethod<?> onWebhookUpdateReceived(Update update) {

        BotApiMethod method = null;

        if (update.hasMessage()) {
            method = updateProcessor.handleUpdate(update);
        }

        if (method != null) {
            try {
                sendApiMethod(method);

            } catch (Exception e) {
                log.error("Ошибка во время обработки апдейта", e);
                sendUserErrorMessage(update.getMessage().getChatId());
            }
        }

        return method;
    }

    @SneakyThrows
    private void sendUserErrorMessage(Long userId) {
        sendApiMethod(SendMessage.builder()
                .chatId(userId)
                .text("Произошла ошибка, попробуйте позже")
                .build());
    }

    @Override
    public String getBotUsername() {
        return "s2tbot";
    }

    @Override
    public String getBotPath() {
        return "/update";
    }
}