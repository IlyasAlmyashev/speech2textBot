package com.almyashev.speech2textbot.telegram;

import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Optional;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Service
public final class LinkPreviewService {
    private static final String PREVIEW_PREFIX = "here's your preview : ";

    private final List<PreviewRule> previewRules = List.of(
            new PreviewRule("https?://(?:www\\.)?instagram\\.com(?=[/?#]|$)", "https://oginstagram.com"),
            new PreviewRule("https?://(?:www\\.)?(?:x\\.com|twitter\\.com)(?=[/?#]|$)", "https://www.fixupx.com"));

    public Optional<String> createPreviewMessage(String text) {
        String previewText = text;
        boolean supportedLinkFound = false;

        for (PreviewRule rule : previewRules) {
            Matcher matcher = rule.sourcePattern().matcher(previewText);
            if (matcher.find()) {
                supportedLinkFound = true;
                previewText = matcher.replaceAll(Matcher.quoteReplacement(rule.previewBaseUrl()));
            }
        }

        return supportedLinkFound
                ? Optional.of(PREVIEW_PREFIX + previewText)
                : Optional.empty();
    }

    private record PreviewRule(Pattern sourcePattern, String previewBaseUrl) {
        private PreviewRule(String sourcePattern, String previewBaseUrl) {
            this(Pattern.compile(sourcePattern, Pattern.CASE_INSENSITIVE), previewBaseUrl);
        }
    }
}
