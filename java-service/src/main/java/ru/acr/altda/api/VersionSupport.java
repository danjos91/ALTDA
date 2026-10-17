package ru.acr.altda.api;

import ru.acr.altda.store.OptimisticLockException;

import java.util.UUID;

public final class VersionSupport {

    private VersionSupport() {
    }

    /**
     * Resolve expected version from {@code If-Match} header and/or body {@code version} field.
     * {@code If-Match} may be a bare number or a quoted etag ({@code "12"}).
     */
    public static Long resolveExpectedVersion(String ifMatch, Long bodyVersion) {
        Long fromHeader = parseIfMatch(ifMatch);
        if (fromHeader != null && bodyVersion != null && !fromHeader.equals(bodyVersion)) {
            throw new IllegalArgumentException("If-Match and body version disagree");
        }
        return fromHeader != null ? fromHeader : bodyVersion;
    }

    public static Long parseIfMatch(String ifMatch) {
        if (ifMatch == null || ifMatch.isBlank() || "*".equals(ifMatch.trim())) {
            return null;
        }
        String raw = ifMatch.trim();
        if (raw.startsWith("W/")) {
            raw = raw.substring(2).trim();
        }
        if (raw.length() >= 2 && raw.startsWith("\"") && raw.endsWith("\"")) {
            raw = raw.substring(1, raw.length() - 1);
        }
        try {
            return Long.parseLong(raw);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException("Invalid If-Match: " + ifMatch);
        }
    }

    public static void requireMatch(UUID id, long actual, Long expected) {
        if (expected != null && actual != expected) {
            throw new OptimisticLockException(id, expected, actual);
        }
    }
}
