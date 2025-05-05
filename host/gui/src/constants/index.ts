const IS_PRODUCTION = process.env.NODE_ENV === "production";
export const LOCAL_API_ROOT = IS_PRODUCTION ? "/api" : "http://localhost:7160/api";