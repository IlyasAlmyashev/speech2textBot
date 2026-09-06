package com.almyashev.speech2textbot;

import java.util.TimeZone;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

import lombok.extern.slf4j.Slf4j;

@Slf4j
@SpringBootApplication
public class Speech2textBotApplication {

    public static void main(String[] args) {
        log.info("Timezone set as {}", TimeZone.getDefault());
        SpringApplication.run(Speech2textBotApplication.class, args);
    }

}
