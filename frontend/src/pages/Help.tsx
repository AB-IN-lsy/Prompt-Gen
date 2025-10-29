/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 13:22:15
 * @FilePath: \electron-go-app\frontend\src\pages\Help.tsx
 * @LastEditTime: 2025-10-11 13:22:15
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import {
    ExternalLink,
    LifeBuoy,
    Mail,
    ClipboardList,
    LayoutDashboard,
    Rocket,
    Settings,
    Library,
    Sparkles
} from "lucide-react";

import { GlassCard } from "../components/ui/glass-card";
import { PageHeader } from "../components/layout/PageHeader";
import { cn } from "../lib/utils";
import { buildCardMotion } from "../lib/animationConfig";

interface HelpSectionContent {
    title: string;
    description: string;
    items?: string[];
}

const sectionIcons = {
    quickStart: Rocket,
    dashboard: LayoutDashboard,
    myPrompts: ClipboardList,
    publicPrompts: Library,
    workbench: Sparkles,
    settings: Settings,
    troubleshooting: LifeBuoy,
    contact: Mail
} as const;

type SectionKey = keyof typeof sectionIcons;

interface HelpResourceLink {
    label: string;
    href?: string;
    type?: "external" | "email";
}

export default function HelpPage() {
    const { t } = useTranslation();

    const sectionKeys = useMemo<SectionKey[]>(() => {
        const order = t("helpPage.sectionOrder", {
            returnObjects: true,
            defaultValue: ["quickStart", "dashboard", "myPrompts", "publicPrompts", "workbench", "settings", "troubleshooting", "contact"]
        }) as string[];
        const keys = order.filter((key): key is SectionKey => key in sectionIcons);
        if (!keys.includes("contact")) {
            keys.push("contact");
        }
        return keys;
    }, [t]);

    const [activeSection, setActiveSection] = useState<SectionKey | null>(() => sectionKeys[0] ?? null);
    const sectionRefs = useRef<Record<SectionKey, HTMLElement | null>>({} as Record<SectionKey, HTMLElement | null>);

    useEffect(() => {
        if (sectionKeys.length === 0) {
            return;
        }
        const observer = new IntersectionObserver(
            (entries) => {
                entries.forEach((entry) => {
                    if (entry.isIntersecting) {
                        const key = entry.target.getAttribute("data-section") as SectionKey | null;
                        if (key) {
                            setActiveSection(key);
                        }
                    }
                });
            },
            {
                rootMargin: "-35% 0px -45% 0px",
                threshold: 0.3,
            },
        );
        sectionKeys.forEach((key) => {
            const node = sectionRefs.current[key];
            if (node) {
                observer.observe(node);
            }
        });
        return () => {
            observer.disconnect();
        };
    }, [sectionKeys]);

    const eyebrow = t("helpPage.eyebrow");
    const title = t("helpPage.title");
    const subtitle = t("helpPage.subtitle");
    const resourcesDescription = t("helpPage.resources.description");
    const resourcesContacts = t("helpPage.resources.contacts", {
        returnObjects: true,
    }) as HelpResourceLink[];
    const resourcesNote = t("helpPage.resources.note");
    const contactTitle = t("helpPage.resources.contactTitle", "联系我");
    const regularSections = useMemo(
        () => sectionKeys.filter((key) => key !== "contact"),
        [sectionKeys],
    );
    const contactActive = activeSection === "contact";
    const resourceIcon = {
        external: ExternalLink,
        email: Mail
    } as const;

    return (
        <div className="space-y-10 text-slate-700 transition-colors dark:text-slate-200">
            <PageHeader eyebrow={eyebrow} title={title} description={subtitle} headingClassName="max-w-3xl" />

            <div className="relative grid gap-8 lg:grid-cols-[240px_1fr]">
                <nav className="hidden lg:block">
                    <div className="sticky top-28 flex flex-col gap-2 text-sm">
                        {sectionKeys.map((key) => {
                            const isContact = key === "contact";
                            const content = isContact
                                ? contactTitle
                                : (t(`helpPage.sections.${key}.title`, {
                                      defaultValue: key,
                                  }) as string);
                            const isActive = activeSection === key;
                            return (
                                <button
                                    key={key}
                                    type="button"
                                    onClick={() => {
                                        setActiveSection(key);
                                        const target = sectionRefs.current[key];
                                        target?.scrollIntoView({ behavior: "smooth", block: "center" });
                                    }}
                                    className={cn(
                                        "flex items-center gap-3 rounded-full px-4 py-2 text-left transition",
                                        isActive
                                            ? "bg-primary/15 text-primary shadow-sm"
                                            : "text-slate-500 hover:bg-white/70 hover:text-primary dark:text-slate-400 dark:hover:bg-slate-800/70",
                                    )}
                                >
                                    <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                                    <span className="truncate text-sm font-medium">{content}</span>
                                </button>
                            );
                        })}
                    </div>
                </nav>

                <div className="relative">
                        {regularSections.map((key, index) => {
                            const Icon = sectionIcons[key];
                            const content = t(`helpPage.sections.${key}`, {
                                returnObjects: true,
                            }) as HelpSectionContent;
                            const isActive = activeSection === key;
                            return (
                                <section
                                    key={key}
                                    ref={(node) => {
                                        sectionRefs.current[key] = node;
                                    }}
                                    data-section={key}
                                    className="relative pb-24"
                                >
                                    <div className="relative" style={{ zIndex: sectionKeys.length - index }}>
                                        <motion.div
                                            {...buildCardMotion({ index, offset: 0 })}
                                            className={cn(
                                                "group relative sticky top-28 overflow-hidden rounded-3xl border border-white/50 bg-white/85 p-6 shadow-lg backdrop-blur-xl transition-all duration-500 dark:border-slate-800/60 dark:bg-slate-900/70",
                                                isActive ? "ring-2 ring-primary/20 shadow-2xl" : "opacity-85",
                                            )}
                                            style={{ top: `calc(6rem + ${index * 2.6}rem)` }}
                                        >
                                            <div className="pointer-events-none absolute inset-0">
                                                <div className="absolute -top-20 left-6 h-48 w-48 rounded-full bg-gradient-to-br from-primary/30 via-sky-300/20 to-transparent blur-3xl opacity-60" />
                                                <div className="absolute -bottom-16 right-6 h-40 w-40 rounded-full bg-gradient-to-tr from-violet-300/25 via-rose-200/20 to-transparent blur-[110px]" />
                                                <div className="absolute inset-0 bg-noise opacity-[0.06]" />
                                            </div>
                                            <div className="relative flex flex-col gap-4">
                                                <div className="flex items-start gap-4">
                                                    <div
                                                        className={cn(
                                                            "rounded-2xl p-3 shadow-inner transition",
                                                            isActive
                                                                ? "bg-primary/15 text-primary"
                                                                : "bg-white/60 text-primary dark:bg-slate-800/60",
                                                        )}
                                                    >
                                                        <Icon className="h-6 w-6" aria-hidden="true" />
                                                    </div>
                                                    <div className="space-y-2">
                                                        <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                                            {content.title}
                                                        </h2>
                                                        <p className="text-sm text-slate-500 dark:text-slate-400">
                                                            {content.description}
                                                        </p>
                                                    </div>
                                                </div>
                                                {content.items && content.items.length > 0 ? (
                                                    <ul className="space-y-2 text-sm text-slate-600 dark:text-slate-300">
                                                        {content.items.map((item) => (
                                                            <li
                                                                key={item}
                                                                className="flex items-start gap-2 rounded-xl bg-white/75 px-4 py-2 shadow-sm transition-colors dark:bg-slate-800/60"
                                                            >
                                                                <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                                                                <span>{item}</span>
                                                            </li>
                                                        ))}
                                                    </ul>
                                                ) : null}
                                            </div>
                                        </motion.div>
                                    </div>
                                </section>
                            );
                        })}

                        <section
                            key="contact"
                            ref={(node) => {
                                sectionRefs.current.contact = node as HTMLElement;
                            }}
                            data-section="contact"
                            className="relative pb-24"
                        >
                            <motion.div
                                {...buildCardMotion({ index: regularSections.length, offset: 0 })}
                                className={cn(
                                    "group relative sticky top-28 overflow-hidden rounded-3xl border border-white/50 bg-white/85 p-6 shadow-lg backdrop-blur-xl transition-all duration-500 dark:border-slate-800/60 dark:bg-slate-900/70",
                                    contactActive ? "ring-2 ring-primary/20 shadow-2xl" : "opacity-85",
                                )}
                            >
                                <div className="pointer-events-none absolute inset-0">
                                    <div className="absolute -top-16 left-4 h-40 w-40 rounded-full bg-gradient-to-br from-primary/25 via-sky-200/25 to-transparent blur-3xl opacity-60" />
                                    <div className="absolute -bottom-16 right-4 h-48 w-48 rounded-full bg-gradient-to-tr from-teal-300/25 via-emerald-300/25 to-transparent blur-[110px]" />
                                    <div className="absolute inset-0 bg-noise opacity-[0.06]" />
                                </div>
                                <div className="relative space-y-3">
                                    <div className="flex items-start gap-3">
                                        <div className="rounded-2xl bg-primary/15 p-3 text-primary shadow-inner">
                                            <Mail className="h-6 w-6" aria-hidden="true" />
                                        </div>
                                        <div className="space-y-2">
                                            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">
                                                {contactTitle}
                                            </h2>
                                            <p className="text-sm text-slate-500 dark:text-slate-400">
                                                {resourcesDescription}
                                            </p>
                                        </div>
                                    </div>
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
                                    {resourcesNote && resourcesNote.length > 0 ? (
                                        <p className="text-xs text-slate-400 dark:text-slate-500">{resourcesNote}</p>
                                    ) : null}
                                </div>
                            </motion.div>
                        </section>
                </div>
            </div>
        </div>
    );
}
