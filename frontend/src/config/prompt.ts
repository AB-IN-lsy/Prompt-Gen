/**
 * Prompt 相关的前端配置项。
 *
 * 通过 `VITE_PROMPT_KEYWORD_LIMIT` 环境变量控制关键词数量上限，
 * 若未配置或填写非法值，则回落到与后端一致的默认 10。
 */
const rawLimit = Number.parseInt(
  import.meta.env.VITE_PROMPT_KEYWORD_LIMIT ?? "",
  10,
);

export const PROMPT_KEYWORD_LIMIT =
  Number.isInteger(rawLimit) && rawLimit > 0 ? rawLimit : 10;

const rawKeywordMaxLength = Number.parseInt(
  import.meta.env.VITE_PROMPT_KEYWORD_MAX_LENGTH ?? "",
  10,
);

export const PROMPT_KEYWORD_MAX_LENGTH =
  Number.isInteger(rawKeywordMaxLength) && rawKeywordMaxLength > 0
    ? rawKeywordMaxLength
    : 32;

const rawTagLimit = Number.parseInt(
  import.meta.env.VITE_PROMPT_TAG_LIMIT ?? "",
  10,
);

export const PROMPT_TAG_LIMIT =
  Number.isInteger(rawTagLimit) && rawTagLimit > 0 ? rawTagLimit : 5;

const rawTagMaxLength = Number.parseInt(
  import.meta.env.VITE_PROMPT_TAG_MAX_LENGTH ?? "",
  10,
);

export const PROMPT_TAG_MAX_LENGTH =
  Number.isInteger(rawTagMaxLength) && rawTagMaxLength > 0
    ? rawTagMaxLength
    : 5;

const rawKeywordRowLimit = Number.parseInt(
  import.meta.env.VITE_KEYWORD_ROW_LIMIT ?? "",
  10,
);

export const KEYWORD_ROW_LIMIT =
  Number.isInteger(rawKeywordRowLimit) && rawKeywordRowLimit > 0
    ? rawKeywordRowLimit
    : 3;

const rawDefaultKeywordWeight = Number.parseInt(
  import.meta.env.VITE_DEFAULT_KEYWORD_WEIGHT ?? "",
  10,
);

export const DEFAULT_KEYWORD_WEIGHT =
  Number.isInteger(rawDefaultKeywordWeight) && rawDefaultKeywordWeight > 0
    ? rawDefaultKeywordWeight
    : 5;

const rawAIGenerateMinDuration = Number.parseInt(
  import.meta.env.VITE_AI_GENERATE_MIN_DURATION_MS ?? "",
  10,
);

export const PROMPT_AI_GENERATE_MIN_DURATION_MS =
  Number.isInteger(rawAIGenerateMinDuration) && rawAIGenerateMinDuration > 0
    ? rawAIGenerateMinDuration
    : 2000;
