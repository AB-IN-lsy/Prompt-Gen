/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-11 13:22:15
 * @FilePath: \electron-go-app\frontend\src\pages\Help.tsx
 * @LastEditTime: 2025-10-11 13:22:15
 */
import { useCallback, useEffect, useMemo, useRef, useState, useId, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import {
    LifeBuoy,
    Mail,
    ClipboardList,
    LayoutDashboard,
    Rocket,
    Settings,
    Library,
    Sparkles,
    Globe2,
    Github
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
    const clipPathId = useId();

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
    const starButtonLabel = t("helpPage.resources.starCta", "Star on GitHub");
    const starCount = t("helpPage.resources.starCount", "6");
    const handleStarClick = useCallback(() => {
        window.open("https://github.com/AB-IN-lsy/Prompt-Gen", "_blank", "noopener,noreferrer");
    }, []);
    const regularSections = useMemo(
        () => sectionKeys.filter((key) => key !== "contact"),
        [sectionKeys],
    );
    const contactActive = activeSection === "contact";
    const resolveContactVisual = (item: HelpResourceLink, index: number) => {
        if (item.type === "email") {
            return {
                icon: Mail,
                gradient: "from-rose-500 to-rose-700",
                border: "border-rose-500/60"
            };
        }
        if ((item.href ?? "").toLowerCase().includes("github")) {
            return {
                icon: Github,
                gradient: "from-gray-900 to-gray-700",
                border: "border-gray-600/60"
            };
        }
        return {
            icon: Globe2,
            gradient: index % 2 === 0 ? "from-amber-500 to-orange-600" : "from-sky-600 to-indigo-700",
            border: index % 2 === 0 ? "border-amber-400/60" : "border-indigo-500/60"
        };
    };

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
                                    <svg width="0" height="0" style={{ position: "absolute" }} aria-hidden="true">
                                        <defs>
                                            <clipPath id={clipPathId} clipPathUnits="objectBoundingBox">
                                                <path d="M 0,0.5 C 0,0 0,0 0.5,0 S 1,0 1,0.5 1,1 0.5,1 0,1 0,0.5" />
                                            </clipPath>
                                        </defs>
                                    </svg>
                                    <div className="relative">
                                        <div className="pointer-events-none absolute inset-0 rounded-3xl border border-white/10 bg-black/15 backdrop-blur-2xl shadow-2xl dark:border-white/5 dark:bg-white/5" />
                                        <div className="relative flex flex-wrap items-center gap-4 p-4">
                                            {resourcesContacts.map((item, index) => {
                                                const visual = resolveContactVisual(item, index);
                                                const Icon = visual.icon;
                                                const shapeStyle: CSSProperties = {
                                                    clipPath: `url(#${clipPathId})`,
                                                    WebkitClipPath: `url(#${clipPathId})`
                                                };
                                                const commonContent = (
                                                    <>
                                                        <div
                                                            style={shapeStyle}
                                                            className={cn(
                                                                "relative grid h-14 w-14 place-items-center rounded-xl bg-gradient-to-br text-white shadow-lg transition-transform duration-300 ease-out",
                                                                visual.gradient,
                                                                visual.border,
                                                                "border cursor-pointer hover:-translate-y-2 hover:scale-110 hover:shadow-2xl active:translate-y-1 active:scale-95"
                                                            )}
                                                        >
                                                            <Icon className="h-7 w-7" aria-hidden="true" />
                                                        </div>
                                                    </>
                                                );
                                                return item.href ? (
                                                    <a
                                                        key={item.label}
                                                        href={item.href}
                                                        target={item.type === "email" ? "_self" : "_blank"}
                                                        rel={item.type === "email" ? undefined : "noreferrer"}
                                                        aria-label={item.label}
                                                        title={item.label}
                                                        className="group relative flex items-center gap-3 rounded-2xl bg-white/70 px-3 py-2 transition-colors hover:bg-white/90 dark:bg-slate-900/70 dark:hover:bg-slate-900/80"
                                                    >
                                                        {commonContent}
                                                    </a>
                                                ) : (
                                                    <span
                                                        key={item.label}
                                                        aria-label={item.label}
                                                        title={item.label}
                                                        className="group relative flex items-center gap-3 rounded-2xl bg-white/70 px-3 py-2 dark:bg-slate-900/70"
                                                    >
                                                        {commonContent}
                                                    </span>
                                                );
                                            })}
                                        </div>
                                    </div>
                                    <div className="flex justify-center pt-4">
                                        <button
                                            type="button"
                                            onClick={handleStarClick}
                                            className="group relative flex w-full max-w-xs items-center justify-center gap-2 overflow-hidden rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white shadow transition-all duration-300 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-900 focus-visible:ring-offset-2 hover:bg-slate-900/90 hover:ring-2 hover:ring-slate-900/80 md:max-w-[14rem]"
                                        >
                                            <span
                                                aria-hidden="true"
                                                className="absolute right-0 -mt-12 h-32 w-8 translate-x-12 rotate-12 bg-white opacity-10 transition-all duration-1000 ease-out group-hover:-translate-x-40"
                                            />
                                            <div className="flex items-center">
                                                <svg className="h-4 w-4 fill-current" viewBox="0 0 438.549 438.549" aria-hidden="true">
                                                    <path d="M409.132 114.573c-19.608-33.596-46.205-60.194-79.798-79.8-33.598-19.607-70.277-29.408-110.063-29.408-39.781 0-76.472 9.804-110.063 29.408-33.596 19.605-60.192 46.204-79.8 79.8C9.803 148.168 0 184.854 0 224.63c0 47.78 13.94 90.745 41.827 128.906 27.884 38.164 63.906 64.572 108.063 79.227 5.14.954 8.945.283 11.419-1.996 2.475-2.282 3.711-5.14 3.711-8.562 0-.571-.049-5.708-.144-15.417a2549.81 2549.81 0 01-.144-25.406l-6.567 1.136c-4.187.767-9.469 1.092-15.846 1-6.374-.089-12.991-.757-19.842-1.999-6.854-1.231-13.229-4.086-19.13-8.559-5.898-4.473-10.085-10.328-12.56-17.556l-2.855-6.57c-1.903-4.374-4.899-9.233-8.992-14.559-4.093-5.331-8.232-8.945-12.419-10.848l-1.999-1.431c-1.332-.951-2.568-2.098-3.711-3.429-1.142-1.331-1.997-2.663-2.568-3.997-.572-1.335-.098-2.43 1.427-3.289 1.525-.859 4.281-1.276 8.28-1.276l5.708.853c3.807.763 8.516 3.042 14.133 6.851 5.614 3.806 10.229 8.754 13.846 14.842 4.38 7.806 9.657 13.754 15.846 17.847 6.184 4.093 12.419 6.136 18.699 6.136 6.28 0 11.704-.476 16.274-1.423 4.565-.952 8.848-2.383 12.847-4.285 1.713-12.758 6.377-22.559 13.988-29.41-10.848-1.14-20.601-2.857-29.264-5.14-8.658-2.286-17.605-5.996-26.835-11.14-9.235-5.137-16.896-11.516-22.985-19.126-6.09-7.614-11.088-17.61-14.987-29.979-3.901-12.374-5.852-26.648-5.852-42.826 0-23.035 7.52-42.637 22.557-58.817-7.044-17.318-6.379-36.732 1.997-58.24 5.52-1.715 13.706-.428 24.554 3.853 10.85 4.283 18.794 7.952 23.84 10.994 5.046 3.041 9.089 5.618 12.135 7.708 17.705-4.947 35.976-7.421 54.818-7.421s37.117 2.474 54.823 7.421l10.849-6.849c7.419-4.57 16.18-8.758 26.262-12.565 10.088-3.805 17.802-4.853 23.134-3.138 8.562 21.509 9.325 40.922 2.279 58.24 15.036 16.18 22.559 35.787 22.559 58.817 0 16.178-1.958 30.497-5.853 42.966-3.9 12.471-8.941 22.457-15.125 29.979-6.191 7.521-13.901 13.85-23.131 18.986-9.232 5.14-18.182 8.85-26.84 11.136-8.662 2.286-18.415 4.004-29.263 5.146 9.894 8.562 14.842 22.077 14.842 40.539v60.237c0 3.422 1.19 6.279 3.572 8.562 2.379 2.279 6.136 2.95 11.276 1.995 44.163-14.653 80.185-41.062 108.068-79.226 27.88-38.161 41.825-81.126 41.825-128.906-.01-39.771-9.818-76.454-29.414-110.049z" />
                                            </svg>
                                                <span className="ml-1">{starButtonLabel}</span>
                                            </div>
                                            <div className="ml-2 flex items-center gap-1 text-sm">
                                                <svg
                                                    className="h-4 w-4 text-slate-400 transition-all duration-300 group-hover:text-yellow-300"
                                                    aria-hidden="true"
                                                    fill="currentColor"
                                                    viewBox="0 0 24 24"
                                                    xmlns="http://www.w3.org/2000/svg"
                                                >
                                                    <path
                                                        clipRule="evenodd"
                                                        fillRule="evenodd"
                                                        d="M10.788 3.21c.448-1.077 1.976-1.077 2.424 0l2.082 5.006 5.404.434c1.164.093 1.636 1.545.749 2.305l-4.117 3.527 1.257 5.273c.271 1.136-.964 2.033-1.96 1.425L12 18.354 7.373 21.18c-.996.608-2.231-.29-1.96-1.425l1.257-5.273-4.117-3.527c-.887-.76-.415-2.212.749-2.305l5.404-.434 2.082-5.005Z"
                                                    />
                                                </svg>
                                                <span className="font-medium tabular-nums">{starCount}</span>
                                            </div>
                                        </button>
                                    </div>
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
