package com.almyashev.speech2textbot.telegram;

import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.telegram.telegrambots.meta.api.methods.BotApiMethod;
import org.telegram.telegrambots.meta.api.methods.ParseMode;
import org.telegram.telegrambots.meta.api.methods.send.SendMessage;
import org.telegram.telegrambots.meta.api.objects.Message;
import org.telegram.telegrambots.meta.api.objects.Update;

import static com.almyashev.speech2textbot.telegram.TelegramBot.ERROR_MESSAGE;

@Slf4j
@Service
@AllArgsConstructor
public class UpdateProcessor {
    private final TelegramAsyncMessageSender telegramAsyncMessageSender;
    private final TelegramVoiceHandler telegramVoiceHandler;

    public BotApiMethod<?> handleUpdate(Update update) {
        var message = update.getMessage();
        log.trace("Message is received. message={}", message);
        var chatId = message.getChatId().toString();

        if (message.hasVoice()) {
            log.info("Start message processing: message={}", message);
            telegramAsyncMessageSender.sendMessageAsync(
                    chatId,
                    () -> handleMessageAsync(message),
                    (throwable) -> getErrorMessage(throwable, chatId)
            );
        }

        return null;
    }

    private SendMessage handleMessageAsync(Message message) {
        SendMessage result = telegramVoiceHandler.processVoice(message);
        result.setParseMode(ParseMode.MARKDOWNV2);
        return result;
    }

    private SendMessage getErrorMessage(Throwable throwable, String chatId) {
        log.error("Произошла ошибка, chatId={}", chatId, throwable);
        return SendMessage.builder()
                .chatId(chatId)
                .text(ERROR_MESSAGE)
                .build();
    }
}
