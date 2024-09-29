package com.almyashev.speech2textbot.telegram;

import com.almyashev.speech2textbot.transcription.TranscribeVoiceToTextService;
import lombok.AllArgsConstructor;
import org.apache.commons.lang3.StringUtils;
import org.springframework.stereotype.Service;
import org.telegram.telegrambots.meta.api.methods.send.SendMessage;
import org.telegram.telegrambots.meta.api.objects.Message;

@Service
@AllArgsConstructor
public class TelegramVoiceHandler {
    private static final String TEMPLATE = "%s said: \n\n%s";
    private static final String AT = "@";
    private static final int MAX_WIDTH = 4096;

    private final TelegramFileService telegramFileService;
    private final TranscribeVoiceToTextService transcribeVoiceToTextService;

    public SendMessage processVoice(Message message) {
        var chatId = message.getChatId();
        var text = StringUtils.abbreviate(
                String.format(
                        TEMPLATE,
                        getAuthor(message),
                        transcribeVoiceToTextService.transcribe(
                                telegramFileService.getFileURL(message.getVoice().getFileId())
                        )),
                MAX_WIDTH
        );

        return new SendMessage(chatId.toString(), text);
    }

    private String getAuthor(Message message) {

        if (message.getForwardFrom() != null) {
            return message.getForwardFrom().getUserName() != null
                    ? AT + message.getForwardFrom().getUserName()
                    : message.getForwardFrom().getFirstName();
        }

        return message.getFrom().getUserName() != null
                ? AT + message.getFrom().getUserName()
                : message.getFrom().getFirstName();
    }
}
