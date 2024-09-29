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
    private static final String TEMPLATE = "@%s said: \n\n%s";
    private static final int MAX_WIDTH = 4096;

    private final TelegramFileService telegramFileService;
    private final TranscribeVoiceToTextService transcribeVoiceToTextService;

    public SendMessage processVoice(Message message) {
        var chatId = message.getChatId();
        var text = StringUtils.abbreviate(
                String.format(TEMPLATE, message.getFrom().getUserName(),
                        transcribeVoiceToTextService.transcribe(
                                telegramFileService.getFile(message.getVoice().getFileId())
                        )),
                MAX_WIDTH
        );

        return new SendMessage(chatId.toString(), text);
    }
}
