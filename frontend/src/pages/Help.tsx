/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 13:22:15
 * @FilePath: \electron-go-app\frontend\src\pages\Help.tsx
 * @LastEditTime: 2025-10-11 13:22:15
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { ExternalLink, LifeBuoy, Mail, KeyRound, Rocket, ShieldCheck, Sparkles } from "lucide-react";

import { GlassCard } from "../components/ui/glass-card";
import { Badge } from "../components/ui/badge";

interface HelpSectionContent {
    title: string;
    description: string;
    items?: string[];
}

const sectionIcons = {
    gettingStarted: Rocket,
    authentication: ShieldCheck,
    workbench: Sparkles,
    models: KeyRound,
    troubleshooting: LifeBuoy
} as const;

type SectionKey = keyof typeof sectionIcons;

interface HelpResourceLink {
    label: string;
    href?: string;
    type?: "external" | "email";
}

export default function HelpPage() {
    const { t } = useTranslation();

    const sectionKeys = useMemo<SectionKey[]>(
        () => ["gettingStarted", "authentication", "workbench", "models", "troubleshooting"],
        []
    );

    const eyebrow = t("helpPage.eyebrow");
    const title = t("helpPage.title");
    const subtitle = t("helpPage.subtitle");
    const resourcesTitle = t("helpPage.resources.title");
    const resourcesDescription = t("helpPage.resources.description");
    const resourcesContacts = t("helpPage.resources.contacts", {
        returnObjects: true
    }) as HelpResourceLink[];
    const resourcesCta = t("helpPage.resources.cta");
    const resourcesNote = t("helpPage.resources.note");
    const resourceIcon = {
        external: ExternalLink,
        email: Mail
    } as const;

    return (
        <div className="space-y-8 text-slate-700 transition-colors dark:text-slate-200">
            <div className="space-y-3">
                <Badge variant="outline">{eyebrow}</Badge>
                <h1 className="text-3xl font-semibold text-slate-900 dark:text-white sm:text-4xl">{title}</h1>
                <p className="max-w-3xl text-sm text-slate-500 dark:text-slate-400 sm:text-base">{subtitle}</p>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
                {sectionKeys.map((key) => {
                    const Icon = sectionIcons[key];
                    const content = t(`helpPage.sections.${key}`, {
                        returnObjects: true
                    }) as HelpSectionContent;

                    return (
                        <GlassCard key={key} className="h-full">
                            <div className="flex items-start gap-4">
                                <div className="rounded-2xl bg-primary/10 p-3 text-primary dark:bg-primary/20">
                                    <Icon className="h-6 w-6" aria-hidden="true" />
                                </div>
                                <div className="space-y-3">
                                    <div>
                                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                            {content.title}
                                        </h2>
                                        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                                            {content.description}
                                        </p>
                                    </div>
                                    {content.items && content.items.length > 0 && (
                                        <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-300">
                                            {content.items.map((item) => (
                                                <li
                                                    key={item}
                                                    className="flex items-start gap-2 rounded-lg bg-white/60 px-3 py-2 dark:bg-slate-800/60"
                                                >
                                                    <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                                                    <span>{item}</span>
                                                </li>
                                            ))}
                                        </ul>
                                    )}
                                </div>
                            </div>
                        </GlassCard>
                    );
                })}
            </div>

            <GlassCard className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
                <div className="space-y-2">
                    <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-50">{resourcesTitle}</h2>
                    <p className="text-sm text-slate-500 dark:text-slate-400">{resourcesDescription}</p>
                    <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-300">
                        {resourcesContacts.map((item) => {
                            const Icon = item.type ? resourceIcon[item.type] ?? ExternalLink : ExternalLink;
                            const content = (
                                <>
                                    <Icon className="h-4 w-4" aria-hidden="true" />
                                    <span>{item.label}</span>
                                </>
                            );
                            return (
                                <li key={item.label}>
                                    {item.href ? (
                                        <a
                                            href={item.href}
                                            target={item.type === "email" ? "_self" : "_blank"}
                                            rel={item.type === "email" ? undefined : "noreferrer"}
                                            className="flex items-center gap-2 rounded-lg bg-white/70 px-3 py-2 transition hover:bg-white/90 dark:bg-slate-800/60 dark:hover:bg-slate-800/80"
                                        >
                                            {content}
                                        </a>
                                    ) : (
                                        <span className="flex items-center gap-2 rounded-lg bg-white/70 px-3 py-2 dark:bg-slate-800/60">
                                            {content}
                                        </span>
                                    )}
                                </li>
                            );
                        })}
                    </ul>
                    <p className="text-xs text-slate-400 dark:text-slate-500">{resourcesNote}</p>
                </div>
                <a
                    href="https://ab-in.blog.csdn.net/"
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center justify-center rounded-full bg-primary px-6 py-2 text-sm font-medium text-white shadow-glow transition hover:shadow-lg focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                >
                    {resourcesCta}
                </a>
            </GlassCard>
        </div>
    );
}
