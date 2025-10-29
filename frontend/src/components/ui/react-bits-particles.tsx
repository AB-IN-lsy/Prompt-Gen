import { useMemo } from "react";
import { motion } from "framer-motion";
import { cn } from "../../lib/utils";

interface ReactBitsParticlesProps {
    className?: string;
}

const parseEnvNumber = (value: string | undefined, fallback: number): number => {
    const parsed = value ? Number.parseFloat(value) : Number.NaN;
    return Number.isFinite(parsed) ? parsed : fallback;
};

const parseEnvInt = (value: string | undefined, fallback: number): number => {
    const parsed = value ? Number.parseInt(value, 10) : Number.NaN;
    return Number.isFinite(parsed) ? parsed : fallback;
};

const FALLBACK_PARTICLE_COUNT = parseEnvInt(
    import.meta.env.VITE_DASHBOARD_PARTICLE_COUNT,
    12
);
const FALLBACK_ORBIT_RADIUS = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_RADIUS,
    220
);
const FALLBACK_ORBIT_DURATION = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_DURATION,
    26
);
const FALLBACK_PARTICLE_SIZE_BASE = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_SIZE_BASE,
    120
);
const FALLBACK_PARTICLE_SIZE_VARIANCE = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_SIZE_VARIANCE,
    60
);
const FALLBACK_PARTICLE_DELAY_STEP = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_DELAY_STEP,
    0.15
);
const FALLBACK_PARTICLE_WAVE_FREQUENCY = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_WAVE_FREQ,
    1.6
);
const FALLBACK_PARTICLE_WAVE_AMPLITUDE = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_WAVE_AMPLITUDE,
    0.45
);
const FALLBACK_PARTICLE_ROTATION_DEGREES = parseEnvNumber(
    import.meta.env.VITE_DASHBOARD_PARTICLE_ROTATION_DEGREES,
    360
);

const GRADIENT_STOPS = [
    "rgba(59,130,246,0.65)",
    "rgba(165,180,252,0.55)",
    "rgba(56,189,248,0.55)",
    "rgba(129,140,248,0.45)"
];

interface ParticleConfig {
    id: string;
    initialAngle: number;
    delay: number;
    size: number;
    gradient: string;
}

export function ReactBitsParticles({ className }: ReactBitsParticlesProps): JSX.Element | null {
    const particleCount = FALLBACK_PARTICLE_COUNT;
    const orbitRadius = FALLBACK_ORBIT_RADIUS;
    const orbitDuration = FALLBACK_ORBIT_DURATION;
    const sizeBase = FALLBACK_PARTICLE_SIZE_BASE;
    const sizeVariance = FALLBACK_PARTICLE_SIZE_VARIANCE;
    const delayStep = FALLBACK_PARTICLE_DELAY_STEP;
    const waveFrequency = FALLBACK_PARTICLE_WAVE_FREQUENCY;
    const waveAmplitude = FALLBACK_PARTICLE_WAVE_AMPLITUDE;
    const rotationDegrees = FALLBACK_PARTICLE_ROTATION_DEGREES;

    const particles = useMemo<ParticleConfig[]>(() => {
        if (particleCount <= 0) {
            return [];
        }
        return Array.from({ length: particleCount }, (_, index) => {
            const progress = particleCount > 1 ? index / (particleCount - 1) : 0;
            const wave = Math.abs(Math.sin(progress * waveFrequency * Math.PI));
            const size = sizeBase + wave * sizeVariance * waveAmplitude;
            const gradientIndex = index % GRADIENT_STOPS.length;
            const gradientColor = GRADIENT_STOPS[gradientIndex];

            return {
                id: `particle-${index}`,
                initialAngle: progress * rotationDegrees,
                delay: index * delayStep,
                size,
                gradient: gradientColor
            };
        });
    }, [
        delayStep,
        particleCount,
        rotationDegrees,
        sizeBase,
        sizeVariance,
        waveAmplitude,
        waveFrequency
    ]);

    if (particles.length === 0) {
        return null;
    }

    return (
        <div className={cn("pointer-events-none absolute inset-0 overflow-hidden", className)}>
            <div className="absolute inset-0">
                {particles.map((particle) => {
                    const sizePx = Number.isFinite(particle.size) ? particle.size : sizeBase;
                    const translateY = -(sizePx / 2);
                    return (
                        <motion.span
                            key={particle.id}
                            className="absolute left-1/2 top-1/2"
                            style={{ transformOrigin: "center" }}
                            initial={{ rotate: particle.initialAngle }}
                            animate={{ rotate: particle.initialAngle + rotationDegrees }}
                            transition={{
                                duration: orbitDuration,
                                ease: "linear",
                                repeat: Infinity,
                                delay: particle.delay,
                                repeatDelay: 0
                            }}
                        >
                            <span
                                className="block rounded-full opacity-70 blur-[72px] will-change-transform"
                                style={{
                                    width: `${sizePx.toFixed(2)}px`,
                                    height: `${sizePx.toFixed(2)}px`,
                                    transform: `translate3d(${orbitRadius}px, ${translateY.toFixed(2)}px, 0)`,
                                    background: `radial-gradient(circle at 35% 35%, rgba(255,255,255,0.7), ${particle.gradient} 55%, transparent 80%)`
                                }}
                            />
                        </motion.span>
                    );
                })}
            </div>
        </div>
    );
}
