package ru.acr.altda.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.scheduling.annotation.AsyncConfigurer;
import org.springframework.scheduling.annotation.EnableAsync;
import org.springframework.scheduling.concurrent.ThreadPoolTaskExecutor;

@Configuration
@EnableAsync
public class AsyncConfig implements AsyncConfigurer {

    public static final String PROGRAM_EXECUTOR = "programTaskExecutor";

    @Bean(name = PROGRAM_EXECUTOR)
    public ThreadPoolTaskExecutor programTaskExecutor() {
        ThreadPoolTaskExecutor executor = new ThreadPoolTaskExecutor();
        // Dedicated platform-thread pool for @Async program jobs (core == max).
        // spring.threads.virtual.enabled applies to the web server, not this executor.
        executor.setCorePoolSize(16);
        executor.setMaxPoolSize(16);
        executor.setQueueCapacity(100);
        executor.setThreadNamePrefix("altda-async-");
        executor.setRejectedExecutionHandler(new java.util.concurrent.ThreadPoolExecutor.AbortPolicy());
        executor.initialize();
        return executor;
    }

    @Override
    public AsyncTaskExecutor getAsyncExecutor() {
        return programTaskExecutor();
    }
}
