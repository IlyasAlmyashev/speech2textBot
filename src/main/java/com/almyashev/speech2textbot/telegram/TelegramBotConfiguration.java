package com.almyashev.speech2textbot.telegram;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.telegram.telegrambots.meta.TelegramBotsApi;
import org.telegram.telegrambots.meta.api.methods.updates.SetWebhook;
import org.telegram.telegrambots.meta.exceptions.TelegramApiException;
import org.telegram.telegrambots.updatesreceivers.DefaultBotSession;

@Configuration
public class TelegramBotConfiguration {
    @Value("${bot.url}")
    private String botUrl;

    @Bean
    public TelegramBotsApi telegramBotsApi(
            TelegramBot telegramBot
    ) throws TelegramApiException {
        var telegramBotsApi = new TelegramBotsApi(DefaultBotSession.class);
        telegramBotsApi.registerBot(
                telegramBot,
                SetWebhook.builder()
                        .url(botUrl)
                        .build()
        );
        return telegramBotsApi;
    }
}