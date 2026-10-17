package ru.acr.altda.api;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import ru.acr.altda.api.dto.ErrorResponse;
import ru.acr.altda.go.GoClientException;
import ru.acr.altda.store.OptimisticLockException;
import ru.acr.altda.store.ProgramNotFoundException;

@RestControllerAdvice
public class ApiExceptionHandler {

    @ExceptionHandler(ProgramNotFoundException.class)
    public ResponseEntity<ErrorResponse> notFound(ProgramNotFoundException ex) {
        return ResponseEntity.status(HttpStatus.NOT_FOUND)
                .body(new ErrorResponse("not_found", ex.getMessage()));
    }

    @ExceptionHandler(ConflictException.class)
    public ResponseEntity<ErrorResponse> conflict(ConflictException ex) {
        String code = ex.getMessage() != null && ex.getMessage().toLowerCase().contains("lock")
                ? GoClientException.NODE_LOCKED
                : "conflict";
        return ResponseEntity.status(HttpStatus.CONFLICT)
                .body(new ErrorResponse(code, ex.getMessage()));
    }

    @ExceptionHandler(GoClientException.class)
    public ResponseEntity<ErrorResponse> goClient(GoClientException ex) {
        // Sync path: surface typed Go codes to the UI. Program is not marked FAILED here
        // (async jobs decide FAILED via GoClientException.shouldMarkProgramFailed).
        return ResponseEntity.status(HttpStatus.valueOf(ex.canonicalHttpStatus()))
                .body(new ErrorResponse(ex.getErrorCode(), ex.getMessage()));
    }

    @ExceptionHandler(OptimisticLockException.class)
    public ResponseEntity<ErrorResponse> optimisticLock(OptimisticLockException ex) {
        return ResponseEntity.status(HttpStatus.CONFLICT)
                .body(new ErrorResponse("version_conflict", ex.getMessage()));
    }

    @ExceptionHandler(IllegalStateException.class)
    public ResponseEntity<ErrorResponse> illegalState(IllegalStateException ex) {
        return ResponseEntity.status(HttpStatus.CONFLICT)
                .body(new ErrorResponse("illegal_state", ex.getMessage()));
    }

    @ExceptionHandler(IllegalArgumentException.class)
    public ResponseEntity<ErrorResponse> badRequest(IllegalArgumentException ex) {
        return ResponseEntity.badRequest()
                .body(new ErrorResponse("bad_request", ex.getMessage()));
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ErrorResponse> validation(MethodArgumentNotValidException ex) {
        String message = ex.getBindingResult().getFieldErrors().stream()
                .findFirst()
                .map(err -> err.getField() + ": " + err.getDefaultMessage())
                .orElse("Validation failed");
        return ResponseEntity.badRequest()
                .body(new ErrorResponse("validation_error", message));
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<ErrorResponse> generic(Exception ex) {
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body(new ErrorResponse("internal_error", ex.getMessage()));
    }
}
