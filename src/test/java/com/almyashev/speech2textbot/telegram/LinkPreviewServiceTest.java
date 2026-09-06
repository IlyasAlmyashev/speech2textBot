package com.almyashev.speech2textbot.telegram;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class LinkPreviewServiceTest {
    private final LinkPreviewService service = new LinkPreviewService();

    @Test
    void createsInstagramPreviewAndPreservesUrlDetails() {
        var result = service.createPreviewMessage(
                "https://www.instagram.com/xxx/?utm_source=test#post");

        assertEquals(
                "here's your preview : https://oginstagram.com/xxx/?utm_source=test#post",
                result.orElseThrow());
    }

    @Test
    void createsXAndTwitterPreviews() {
        var result = service.createPreviewMessage(
                "X: https://www.x.com/x and Twitter: https://twitter.com/user/status/1");

        assertEquals(
                "here's your preview : X: https://www.fixupx.com/x and Twitter: "
                        + "https://www.fixupx.com/user/status/1",
                result.orElseThrow());
    }

    @Test
    void ignoresUnsupportedLinks() {
        assertTrue(service.createPreviewMessage("https://example.com/post").isEmpty());
    }
}
