package com.almyashev.speech2textbot.common;

import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import javax.annotation.PreDestroy;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;

@Slf4j
@Service
public class AsyncOperationService {
    private static final int DEFAULT_MAX_RETRIES = 3;
    private static final int DEFAULT_RETRY_DELAY_MS = 5000; // 5 seconds
    private static final int MIN_THREADS = 4;

    private final ExecutorService executorService;

    public AsyncOperationService(
    ) {
        int threadCount = calculateOptimalThreads();
        log.info("Initializing ExecutorService with {} threads", threadCount);
        this.executorService = Executors.newFixedThreadPool(threadCount);
    }

    public <T> CompletableFuture<T> executeAsync(
            Supplier<T> operation,
            String operationName
    ) {
        return executeWithRetry(operation, operationName, DEFAULT_MAX_RETRIES);
    }

    public <T> CompletableFuture<T> executeWithRetry(
            Supplier<T> operation,
            String operationName,
            int maxRetries
    ) {
        CompletableFuture<T> future = new CompletableFuture<>();
        retryOperation(operation, operationName, maxRetries, 0, future);
        return future;
    }

    private <T> void retryOperation(
            Supplier<T> operation,
            String operationName,
            int maxRetries,
            int attempt,
            CompletableFuture<T> future
    ) {
        CompletableFuture.supplyAsync(operation, executorService)
                .thenAccept(future::complete)
                .exceptionally(throwable -> {
                    if (attempt >= maxRetries - 1) {
                        log.error("[{}] All retry attempts failed", operationName, throwable);
                        future.completeExceptionally(throwable);
                        return null;
                    }

                    log.warn("[{}] Attempt {} failed: {}", operationName, attempt + 1, throwable.getMessage());
                    CompletableFuture.delayedExecutor(DEFAULT_RETRY_DELAY_MS, TimeUnit.MILLISECONDS, executorService)
                            .execute(() -> retryOperation(operation, operationName, maxRetries, attempt + 1, future));
                    return null;
                });
    }

    // Метод для завершения работы пула потоков
    // при остановке приложения
    @PreDestroy
    public void shutdown() {
        executorService.shutdown();
        try {
            if (!executorService.awaitTermination(60, TimeUnit.SECONDS)) {
                executorService.shutdownNow();
            }
        } catch (InterruptedException e) {
            executorService.shutdownNow();
            Thread.currentThread().interrupt();
        }
    }

    // Метод для вычисления оптимального количества потоков
    // в зависимости от конфигурации и доступных ресурсов
    private int calculateOptimalThreads() {

        // Получаем количество доступных процессоров
        int cpuCores = Runtime.getRuntime().availableProcessors();
        log.info("Available CPU cores: {}", cpuCores);

        // Для I/O-bound операций умножаем на коэффициент
        int optimal = cpuCores * 4;

        // Ограничиваем минимальное и максимальное значение
        return Math.max(MIN_THREADS, optimal);
    }

    // Метод для мониторинга состояния пула потоков
    @Scheduled(fixedRate = 60000) // Каждую минуту
    public void monitorThreadPool() {
        if (executorService instanceof ThreadPoolExecutor executor) {
            log.info("Thread pool stats: active={}, queue={}, completed={}",
                    executor.getActiveCount(),
                    executor.getQueue().size(),
                    executor.getCompletedTaskCount());
        }
    }
}
