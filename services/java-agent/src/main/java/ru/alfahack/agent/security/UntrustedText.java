package ru.alfahack.agent.security;

import java.util.regex.MatchResult;
import java.util.regex.Pattern;

public final class UntrustedText {

    private static final int MAX_LENGTH = 20_000;
    private static final Pattern CONTROL_CHARS = Pattern.compile("[\\x00-\\x08\\x0B\\x0C\\x0E-\\x1F\\x7F]");
    private static final Pattern BLOCK_OPEN = Pattern.compile("<{3,}");
    private static final Pattern BLOCK_CLOSE = Pattern.compile(">{3,}");
    private static final Pattern DELIMITER_TOKEN = Pattern.compile(
            "(?i)(STORY|COMPANY|TOOLS|DRAFT|FAILURES|BANK|QUESTION|INSIGHT)\\s*;");

    private UntrustedText() {
    }

    public static String sanitize(String raw) {
        if (raw == null) {
            return "";
        }
        String out = CONTROL_CHARS.matcher(raw).replaceAll(" ");
        out = BLOCK_OPEN.matcher(out).replaceAll("‹‹‹");
        out = BLOCK_CLOSE.matcher(out).replaceAll("›››");
        out = DELIMITER_TOKEN.matcher(out).replaceAll(UntrustedText::weakenDelimiter);
        return out.length() > MAX_LENGTH ? out.substring(0, MAX_LENGTH) : out;
    }

    private static String weakenDelimiter(MatchResult match) {
        return match.group().replace(';', ',');
    }
}
