package ru.acr.altda.api;

/** Returned as HTTP 409 when the program is busy generating/validating or otherwise not editable. */
public class ConflictException extends RuntimeException {

    public ConflictException(String message) {
        super(message);
    }
}
