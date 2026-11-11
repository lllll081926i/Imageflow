// Package brand holds the application's identity: the logo document and the
// project's public URLs.
//
// It is a package of its own because more than the UI needs the mark: the icon
// generator writes resources/icon.png from the same bytes the window draws, so
// the installer, the taskbar, the tray and the in-app logo can never drift into
// three different pictures.
package brand

// LogoSVG is the original project's frontend/logo.svg, unchanged.
//
// The mark is a stroked rounded square (112..912 of a 1024 canvas) with two arcs
// and a centre dot, filled by a blue → indigo → purple gradient. The generous
// margin is what the original tile had; IconViewBox below trims it for icons,
// where an operating system draws the mark at 16 to 256 pixels and every unused
// pixel of margin makes the mark look smaller.
const LogoSVG = `<svg width="1024" height="1024" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
        <linearGradient id="flow_gradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stop-color="#007AFF"/>
            <stop offset="50%" stop-color="#5856D6"/>
            <stop offset="100%" stop-color="#AF52DE"/>
        </linearGradient>
    </defs>
    <rect x="112" y="112" width="800" height="800" rx="200" fill="white" fill-opacity="0.05"/>
    <rect x="112" y="112" width="800" height="800" rx="200" stroke="url(#flow_gradient)" stroke-width="32"/>
    <path d="M360 512C360 423.634 431.634 352 520 352H660" stroke="url(#flow_gradient)" stroke-width="80" stroke-linecap="round"/>
    <path d="M660 512C660 600.366 588.366 672 500 672H360" stroke="url(#flow_gradient)" stroke-width="80" stroke-linecap="round"/>
    <circle cx="512" cy="512" r="120" fill="url(#flow_gradient)" fill-opacity="0.1"/>
    <circle cx="512" cy="512" r="60" fill="url(#flow_gradient)"/>
</svg>`

// IconSVG is LogoSVG with its viewBox trimmed to the mark's own bounds.
//
// The rounded square spans 112..912 and its 32-wide stroke puts the outer edge
// at 96..928, so a viewBox of 96 96 832 832 renders the mark edge to edge. That
// is what makes a 16×16 title-bar or taskbar icon readable: the same shape, 20%
// larger, with no dead margin around it.
const IconSVG = `<svg width="1024" height="1024" viewBox="96 96 832 832" fill="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
        <linearGradient id="flow_gradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stop-color="#007AFF"/>
            <stop offset="50%" stop-color="#5856D6"/>
            <stop offset="100%" stop-color="#AF52DE"/>
        </linearGradient>
    </defs>
    <rect x="112" y="112" width="800" height="800" rx="200" fill="white" fill-opacity="0.05"/>
    <rect x="112" y="112" width="800" height="800" rx="200" stroke="url(#flow_gradient)" stroke-width="32"/>
    <path d="M360 512C360 423.634 431.634 352 520 352H660" stroke="url(#flow_gradient)" stroke-width="80" stroke-linecap="round"/>
    <path d="M660 512C660 600.366 588.366 672 500 672H360" stroke="url(#flow_gradient)" stroke-width="80" stroke-linecap="round"/>
    <circle cx="512" cy="512" r="120" fill="url(#flow_gradient)" fill-opacity="0.1"/>
    <circle cx="512" cy="512" r="60" fill="url(#flow_gradient)"/>
</svg>`

// RepoURL is the project's public GitHub repository. The About page opens it and
// the release feed publishes to it.
const RepoURL = "https://github.com/lllll081926i/Imageflow"

// RepoLabel is how that repository reads in the UI.
const RepoLabel = "lllll081926i/Imageflow"
