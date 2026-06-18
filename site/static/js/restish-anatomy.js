(function () {
  "use strict";

  function initAnatomyBlock(block) {
    if (block.dataset.restishAnatomyReady === "true") {
      return;
    }
    block.dataset.restishAnatomyReady = "true";

    const targets = Array.from(block.querySelectorAll("[data-anatomy-part]"));
    const connectorPath = block.querySelector("[data-restish-anatomy-string]");
    const connectorEnabled = connectorPath && !prefersReducedMotion() && !usesCoarsePointer();
    let activePart = "";
    let activeToken = null;
    let activeCard = null;
    let startPoint = null;
    let endPoint = null;
    let pointerPoint = null;
    let previousPointerPoint = null;
    let pointerControls = "";
    let pointerImpulse = 0;
    let stringOffset = 0;
    let stringVelocity = 0;
    let previousFrame = 0;
    let animationFrame = 0;

    function activate(part, source) {
      if (!part) {
        clear();
        return;
      }

      activePart = part;
      activeToken = findAnatomyPart(block, part, ".restish-anatomy__command");
      activeCard = findAnatomyPart(block, part, ".restish-anatomy__grid");
      block.dataset.activeAnatomy = part;
      targets.forEach(function (target) {
        target.toggleAttribute("data-anatomy-active", target.dataset.anatomyPart === part);
      });

      const activeColor = window.getComputedStyle(source).getPropertyValue("--anatomy-color").trim();
      if (activeColor) {
        block.style.setProperty("--anatomy-active-color", activeColor);
      }

      if (!connectorEnabled || !activeToken || !activeCard) {
        block.dataset.connectorActive = "false";
        return;
      }

      previousFrame = 0;
      pointerControls = source.classList.contains("restish-anatomy__item") ? "card" : "token";
      stringVelocity += pointerControls === "card" ? -13 : 13;
      updateConnectorAnchors();
      block.dataset.connectorActive = "true";
      drawConnector();
      if (!animationFrame) {
        animationFrame = window.requestAnimationFrame(animateConnector);
      }
    }

    function clear() {
      activePart = "";
      activeToken = null;
      activeCard = null;
      startPoint = null;
      endPoint = null;
      pointerPoint = null;
      previousPointerPoint = null;
      pointerControls = "";
      pointerImpulse = 0;
      previousFrame = 0;
      stringOffset = 0;
      stringVelocity = 0;
      delete block.dataset.activeAnatomy;
      block.dataset.connectorActive = "false";
      targets.forEach(function (target) {
        target.removeAttribute("data-anatomy-active");
      });
      if (connectorPath) {
        connectorPath.setAttribute("d", "");
      }
      if (animationFrame) {
        window.cancelAnimationFrame(animationFrame);
        animationFrame = 0;
      }
    }

    function updateConnectorAnchors() {
      if (!activeToken || !activeCard || !connectorPath) {
        return;
      }
      const rect = block.getBoundingClientRect();
      const tokenRect = activeToken.getBoundingClientRect();
      const cardRect = activeCard.getBoundingClientRect();
      const svg = connectorPath.ownerSVGElement;
      svg.setAttribute("viewBox", `0 0 ${rect.width} ${rect.height}`);
      startPoint = {
        x: tokenRect.left + tokenRect.width / 2 - rect.left,
        y: tokenRect.bottom - rect.top + 3
      };
      endPoint = {
        x: cardRect.left + cardRect.width / 2 - rect.left,
        y: cardRect.top - rect.top - 3
      };
    }

    function updatePointer(event) {
      if (!connectorEnabled || !activePart) {
        return;
      }
      const rect = block.getBoundingClientRect();
      const nextPoint = {
        x: event.clientX - rect.left,
        y: event.clientY - rect.top
      };
      previousPointerPoint = pointerPoint;
      pointerPoint = nextPoint;
      updateConnectorAnchors();
      addPointerImpulse();
      drawConnector();
      if (!animationFrame) {
        previousFrame = 0;
        animationFrame = window.requestAnimationFrame(animateConnector);
      }
    }

    function currentStartPoint() {
      return pointerControls === "token" && pointerPoint ? pointerPoint : startPoint;
    }

    function currentEndPoint() {
      return pointerControls === "card" && pointerPoint ? pointerPoint : endPoint;
    }

    function addPointerImpulse() {
      if (!previousPointerPoint || !startPoint || !endPoint) {
        return;
      }
      const from = currentStartPoint();
      const to = currentEndPoint();
      if (!from || !to) {
        return;
      }
      const dx = to.x - from.x;
      const dy = to.y - from.y;
      const length = Math.max(Math.hypot(dx, dy), 1);
      const normalX = -dy / length;
      const normalY = dx / length;
      const moveX = pointerPoint.x - previousPointerPoint.x;
      const moveY = pointerPoint.y - previousPointerPoint.y;
      const perpendicularMovement = moveX * normalX + moveY * normalY;
      const movement = Math.hypot(moveX, moveY);
      if (movement < 0.2) {
        return;
      }
      const direction = pointerControls === "card" ? -1 : 1;
      const impulse = clamp(perpendicularMovement * 0.62 * direction, -18, 18);
      pointerImpulse = clamp(pointerImpulse + impulse, -30, 30);
      stringVelocity = clamp(stringVelocity + impulse * 0.95, -28, 28);
    }

    function drawConnector() {
      if (!startPoint || !endPoint || !connectorPath) {
        return;
      }
      const from = currentStartPoint();
      const to = currentEndPoint();
      if (!from || !to) {
        return;
      }
      const dx = to.x - from.x;
      const dy = to.y - from.y;
      const length = Math.max(Math.hypot(dx, dy), 1);
      const normalX = -dy / length;
      const normalY = dx / length;
      const bend = stringOffset + pointerImpulse;
      const controlX = (from.x + to.x) / 2 + normalX * bend;
      const controlY = (from.y + to.y) / 2 + normalY * bend;
      connectorPath.setAttribute(
        "d",
        `M ${from.x.toFixed(1)} ${from.y.toFixed(1)} Q ${controlX.toFixed(1)} ${controlY.toFixed(1)} ${to.x.toFixed(1)} ${to.y.toFixed(1)}`
      );
    }

    function animateConnector(timestamp) {
      animationFrame = 0;
      if (!activePart || !startPoint || !endPoint || !connectorPath) {
        return;
      }

      if (!previousFrame) {
        previousFrame = timestamp;
      }
      const delta = Math.min((timestamp - previousFrame) / 16.67, 2);
      previousFrame = timestamp;

      stringVelocity += (0 - stringOffset) * 0.18 * delta;
      stringVelocity *= Math.pow(0.79, delta);
      stringOffset += stringVelocity * delta;
      pointerImpulse *= Math.pow(0.72, delta);
      drawConnector();

      if (Math.abs(stringVelocity) > 0.035 || Math.abs(stringOffset) > 0.12 || Math.abs(pointerImpulse) > 0.12) {
        animationFrame = window.requestAnimationFrame(animateConnector);
      } else {
        stringOffset = 0;
        stringVelocity = 0;
        pointerImpulse = 0;
        drawConnector();
      }
    }

    targets.forEach(function (target) {
      target.addEventListener("mouseenter", function () {
        activate(target.dataset.anatomyPart, target);
      });
      target.addEventListener("focus", function () {
        activate(target.dataset.anatomyPart, target);
      });
      target.addEventListener("mousemove", updatePointer);
    });

    block.addEventListener("mouseleave", clear);
    block.addEventListener("focusout", function (event) {
      if (!block.contains(event.relatedTarget)) {
        clear();
      }
    });
    window.addEventListener("resize", function () {
      if (activePart) {
        updateConnectorAnchors();
      }
    });
  }

  function findAnatomyPart(block, part, scopeSelector) {
    const scope = block.querySelector(scopeSelector);
    if (!scope) {
      return null;
    }
    return Array.from(scope.querySelectorAll("[data-anatomy-part]")).find(function (item) {
      return item.dataset.anatomyPart === part;
    }) || null;
  }

  function prefersReducedMotion() {
    return Boolean(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  }

  function usesCoarsePointer() {
    return Boolean(window.matchMedia && window.matchMedia("(hover: none), (pointer: coarse)").matches);
  }

  function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
  }

  function initAllAnatomyBlocks() {
    document.querySelectorAll("[data-restish-anatomy]").forEach(initAnatomyBlock);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initAllAnatomyBlocks);
  } else {
    initAllAnatomyBlocks();
  }
})();
