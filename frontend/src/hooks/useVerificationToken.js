/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 22:48:39
 * @FilePath: \electron-go-app\frontend\src\hooks\useVerificationToken.ts
 * @LastEditTime: 2025-10-10 22:48:45
 */
import { useEffect, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
export function useVerificationToken() {
    const [searchParams, setSearchParams] = useSearchParams();
    const token = useMemo(() => searchParams.get("verificationToken"), [searchParams]);
    useEffect(() => {
        if (!token) {
            return;
        }
        const next = new URLSearchParams(searchParams);
        next.delete("verificationToken");
        setSearchParams(next, { replace: true });
    }, [token, searchParams, setSearchParams]);
    return token;
}
